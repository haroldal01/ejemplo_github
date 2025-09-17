package UserManager

import (
	"MIA_P1/DiskManagement"
	"MIA_P1/OutPut"
	"MIA_P1/Structs"
	"MIA_P1/Utilities"
	"MIA_P1/stores"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

var currentUser struct {
	loggedIn  bool
	user      string
	partition string // ID de la partición
}

func normalizePath(path string) string {
	// Si la ruta contiene backslashes (\), los convertimos a /
	path = strings.ReplaceAll(path, "\\", "/")

	// Si empieza con una letra y dos puntos, como C:/...
	if len(path) >= 2 && path[1] == ':' {
		drive := strings.ToUpper(string(path[0]))
		path = "/" + drive + path[2:] // convierte "C:/..." → "/C/..."
	}

	// Si no empieza con '/', lo agregamos
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return path
}

func SearchPath(path string, createParents bool, currentPartition *DiskManagement.MountedPartition) int {
	if path == "/" {
		return 0
	}
	components := strings.Split(path, "/")[1:]
	// Abrir el disco.
	file, err := Utilities.OpenFile(currentPartition.Path)
	if err != nil {
		return -1
	}
	defer file.Close()
	// Obtener MBR y Superblock.
	var TempMBR Structs.MRB
	if err := Utilities.ReadObject(file, &TempMBR, 0); err != nil {
		return -1
	}
	var partIndex int = -1
	for i := 0; i < 4; i++ {
		if strings.Contains(string(TempMBR.Partitions[i].Id[:]), currentPartition.ID) {
			partIndex = i
			break
		}
	}
	if partIndex == -1 {
		return -1
	}
	var sb Structs.Superblock
	if err := Utilities.ReadObject(file, &sb, int64(TempMBR.Partitions[partIndex].Start)); err != nil {
		return -1
	}
	currentIndex := 0
	for _, comp := range components {
		inode, _ := GetInodeFromPathByIndex(currentIndex, file, sb)
		folder, err := ReadFolderBlock(file, sb, inode.I_block[0])
		if err != nil {
			return -1
		}
		found := false
		for _, entry := range folder.B_content {
			if strings.Trim(string(entry.B_name[:]), "\x00") == comp {
				currentIndex = int(entry.B_inodo)
				found = true
				break
			}
		}
		if !found {
			if createParents {
				// Crear la carpeta faltante.
				newInode, _, newIndex, err := allocateInode(file, sb, currentUser.user, "default", "664", true)
				if err != nil {
					return -1
				}
				// Inicializar la carpeta con "." y "..".
				if err := InitializeFolder(newInode, nil, newIndex, currentIndex); err != nil {
					return -1
				}
				// Agregar la entrada en el FolderBlock del directorio actual.
				if err := AddEntryToFolderByIndex(currentIndex, file, sb, comp, newIndex); err != nil {
					return -1
				}
				currentIndex = newIndex
			} else {
				return -1
			}
		}
	}
	return currentIndex
}

func EntryExistsInFolder(folderInode Structs.Inode, file *os.File, sb Structs.Superblock, name string) bool {
	folder, err := ReadFolderBlock(file, sb, folderInode.I_block[0])
	if err != nil {
		return false
	}
	for _, entry := range folder.B_content {
		if strings.Trim(string(entry.B_name[:]), "\x00") == name {
			return true
		}
	}
	return false
}

// MultiBlockUpdateFile actualiza el contenido completo de un archivo distribuyéndolo en bloques.
// Soporta 12 apuntadores directos y un indirecto simple (I_block[12]).
// inodeOffset: offset en disco del inodo.
func MultiBlockUpdateFile(inode *Structs.Inode, fullData string, file *os.File, sb Structs.Superblock, inodeOffset int64) error {
	blockSize := binary.Size(Structs.Fileblock{})
	// Calcula el número de bloques requeridos.
	requiredBlocks := (len(fullData) + blockSize - 1) / blockSize
	directLimit := 12

	// Escribir en bloques directos.
	for i := 0; i < requiredBlocks && i < directLimit; i++ {
		start := i * blockSize
		end := start + blockSize
		if end > len(fullData) {
			end = len(fullData)
		}
		chunk := fullData[start:end]
		if inode.I_block[i] == -1 {
			blk, err := allocateBlock(file, sb)
			if err != nil {
				return fmt.Errorf("no se pudo asignar bloque directo para chunk %d: %v", i, err)
			}
			inode.I_block[i] = blk
		}
		var block Structs.Fileblock
		copy(block.B_content[:], chunk)
		blockOffset := int64(sb.S_block_start) + int64(inode.I_block[i])*int64(blockSize)
		fmt.Printf("MultiBlockUpdateFile: Escribiendo bloque directo %d en offset %d, bloque ID %d\n", i, blockOffset, inode.I_block[i])
		if err := Utilities.WriteObject(file, block, blockOffset); err != nil {
			return fmt.Errorf("error al escribir bloque directo %d: %v", i, err)
		}
	}

	// Si solo se necesitan bloques directos:
	if requiredBlocks <= directLimit {
		// Liberar bloques directos sobrantes.
		for i := requiredBlocks; i < directLimit; i++ {
			if inode.I_block[i] != -1 {
				var emptyBlock Structs.Fileblock
				blockOffset := int64(sb.S_block_start) + int64(inode.I_block[i])*int64(blockSize)
				Utilities.WriteObject(file, emptyBlock, blockOffset)
				freeBlockInBitmap(file, sb, inode.I_block[i])
				inode.I_block[i] = -1
			}
		}
		inode.I_size = int32(len(fullData))
		return Utilities.WriteObject(file, *inode, inodeOffset)
	}

	// Usar apuntador indirecto en I_block[12].
	remaining := requiredBlocks - directLimit
	// Se asume que el bloque indirecto puede contener, por ejemplo, 16 apuntadores.
	maxIndirect := 16
	if remaining > maxIndirect {
		return fmt.Errorf("archivo excede la capacidad soportada (requiere %d bloques, máximo %d)", requiredBlocks, directLimit+maxIndirect)
	}
	// Asignar bloque para tabla de punteros indirectos, si no está asignado.
	if inode.I_block[directLimit] == -1 {
		blk, err := allocateBlock(file, sb)
		if err != nil {
			return fmt.Errorf("no se pudo asignar el bloque indirecto: %v", err)
		}
		inode.I_block[directLimit] = blk
	}
	indirectBlockIndex := inode.I_block[directLimit]
	// Leer la tabla de punteros indirectos.
	var ptrBlock Structs.Pointerblock
	ptrBlockSize := binary.Size(Structs.Pointerblock{})
	ptrBlockOffset := int64(sb.S_block_start) + int64(indirectBlockIndex)*int64(ptrBlockSize)
	if err := Utilities.ReadObject(file, &ptrBlock, ptrBlockOffset); err != nil {
		// Si no se puede leer, inicializar con -1.
		for j := 0; j < len(ptrBlock.B_pointers); j++ {
			ptrBlock.B_pointers[j] = -1
		}
	}

	// Escribir en bloques indirectos.
	for j := 0; j < remaining; j++ {
		i := directLimit + j
		start := i * blockSize
		end := start + blockSize
		if end > len(fullData) {
			end = len(fullData)
		}
		chunk := fullData[start:end]
		if ptrBlock.B_pointers[j] == -1 {
			blk, err := allocateBlock(file, sb)
			if err != nil {
				return fmt.Errorf("no se pudo asignar bloque indirecto para chunk %d: %v", j, err)
			}
			ptrBlock.B_pointers[j] = blk
		}
		var block Structs.Fileblock
		copy(block.B_content[:], chunk)
		blockOffset := int64(sb.S_block_start) + int64(ptrBlock.B_pointers[j])*int64(blockSize)
		fmt.Printf("MultiBlockUpdateFile: Escribiendo bloque indirecto %d en offset %d, bloque ID %d\n", j, blockOffset, ptrBlock.B_pointers[j])
		if err := Utilities.WriteObject(file, block, blockOffset); err != nil {
			return fmt.Errorf("error al escribir bloque indirecto %d: %v", j, err)
		}
	}
	// Escribir la tabla de punteros indirectos en disco.
	if err := Utilities.WriteObject(file, ptrBlock, ptrBlockOffset); err != nil {
		return fmt.Errorf("error al escribir la tabla de punteros indirectos: %v", err)
	}

	// Liberar bloques indirectos sobrantes.
	for j := remaining; j < len(ptrBlock.B_pointers); j++ {
		if ptrBlock.B_pointers[j] != -1 {
			var emptyBlock Structs.Fileblock
			blockOffset := int64(sb.S_block_start) + int64(ptrBlock.B_pointers[j])*int64(blockSize)
			Utilities.WriteObject(file, emptyBlock, blockOffset)
			freeBlockInBitmap(file, sb, ptrBlock.B_pointers[j])
			ptrBlock.B_pointers[j] = -1
		}
	}
	inode.I_size = int32(len(fullData))
	return Utilities.WriteObject(file, *inode, inodeOffset)
}

// AddEntryToFolderByIndex agrega una entrada en el FolderBlock de la carpeta cuyo inodo está en parentIndex.
// Se utiliza el nuevo inodo (newIndex) para la nueva entrada.
func AddEntryToFolderByIndex(parentIndex int, file *os.File, sb Structs.Superblock, entryName string, newIndex int) error {
	parentInode, _ := GetInodeFromPathByIndex(parentIndex, file, sb)
	if parentInode == nil {
		return fmt.Errorf("no se encontró la carpeta padre (inodo %d)", parentIndex)
	}
	// Asegurarse de que el FolderBlock esté asignado.
	if parentInode.I_block[0] == -1 {
		blk, err := allocateBlock(file, sb)
		if err != nil {
			return fmt.Errorf("error al asignar bloque para el FolderBlock: %v", err)
		}
		parentInode.I_block[0] = blk
		// Actualizar el inodo padre.
		inodeSize := binary.Size(Structs.Inode{})
		offset := int64(sb.S_inode_start) + int64(parentIndex)*int64(inodeSize)
		if err := Utilities.WriteObject(file, *parentInode, offset); err != nil {
			return fmt.Errorf("error al actualizar el inodo padre: %v", err)
		}
	}
	folder, err := ReadFolderBlock(file, sb, parentInode.I_block[0])
	if err != nil {
		return fmt.Errorf("error al leer el FolderBlock: %v", err)
	}
	// Buscar entrada vacía.
	for i := 0; i < len(folder.B_content); i++ {
		if strings.Trim(string(folder.B_content[i].B_name[:]), "\x00") == "" {
			copy(folder.B_content[i].B_name[:], entryName)
			folder.B_content[i].B_inodo = int32(newIndex)
			blockSize := binary.Size(Structs.Folderblock{})
			offset := int64(sb.S_block_start) + int64(parentInode.I_block[0])*int64(blockSize)
			if err := Utilities.WriteObject(file, folder, offset); err != nil {
				return fmt.Errorf("error al escribir el FolderBlock actualizado: %v", err)
			}
			fmt.Printf("AddEntryToFolderByIndex: Se agregó la entrada '%s' con inodo %d\n", entryName, newIndex)
			return nil
		}
	}
	return fmt.Errorf("no hay espacio en la carpeta para una nueva entrada")
}

// ReadFolderBlock lee un FolderBlock dado el índice de bloque.
func ReadFolderBlock(file *os.File, sb Structs.Superblock, blockIndex int32) (*Structs.Folderblock, error) {
	blockSize := binary.Size(Structs.Folderblock{})
	offset := int64(sb.S_block_start) + int64(blockIndex)*int64(blockSize)
	var folder Structs.Folderblock
	if err := Utilities.ReadObject(file, &folder, offset); err != nil {
		return nil, err
	}
	return &folder, nil
}

// GetInodeFromPathByIndex retorna el inodo y su offset dado un índice.
func GetInodeFromPathByIndex(index int, file *os.File, sb Structs.Superblock) (*Structs.Inode, int64) {
	inodeSize := binary.Size(Structs.Inode{})
	offset := int64(sb.S_inode_start) + int64(index)*int64(inodeSize)
	var inode Structs.Inode
	if err := Utilities.ReadObject(file, &inode, offset); err != nil {
		return nil, 0
	}
	return &inode, offset
}

// allocateInode recorre la tabla de inodos y asigna el primer inodo libre.
// Retorna un puntero al inodo, su offset en disco, el índice asignado y error.
func allocateInode(file *os.File, sb Structs.Superblock, owner, group, perm string, isDirectory bool) (*Structs.Inode, int64, int, error) {
	inodeSize := binary.Size(Structs.Inode{})
	for i := 0; i < int(sb.S_inodes_count); i++ {
		offset := int64(sb.S_inode_start) + int64(i)*int64(inodeSize)
		var inode Structs.Inode
		if err := Utilities.ReadObject(file, &inode, offset); err != nil {
			return nil, 0, -1, err
		}
		// Un inodo libre se identifica con I_type[0] == 0.
		if inode.I_type[0] == 0 {
			now := time.Now().Format("02/01/2006 15:04")
			inode.I_uid = 1 // Aquí puedes mapear el nombre owner a un ID real.
			inode.I_gid = 1 // Similar para group.
			inode.I_size = 0
			copy(inode.I_atime[:], now)
			copy(inode.I_ctime[:], now)
			copy(inode.I_mtime[:], now)
			// Inicializar todos los 15 apuntadores a -1.
			for j := 0; j < 15; j++ {
				inode.I_block[j] = -1
			}
			if isDirectory {
				inode.I_type[0] = '0'
			} else {
				inode.I_type[0] = '1'
			}
			// Asignar permisos (ej. "664")
			if len(perm) >= 3 {
				copy(inode.I_perm[:], perm[:3])
			} else {
				copy(inode.I_perm[:], "664")
			}
			// Escribir el inodo actualizado.
			if err := Utilities.WriteObject(file, inode, offset); err != nil {
				return nil, 0, -1, err
			}
			fmt.Printf("allocateInode: Inodo asignado en índice %d, offset %d, I_block: %v\n", i, offset, inode.I_block)
			return &inode, offset, i, nil
		}
	}
	return nil, 0, -1, fmt.Errorf("no hay inodos libres")
}

func InitializeFolder(newFolderInode *Structs.Inode, parentInode *Structs.Inode, newIndex, parentIndex int) error {
	var folder Structs.Folderblock
	// Entrada "." apunta al propio directorio.
	copy(folder.B_content[0].B_name[:], ".")
	folder.B_content[0].B_inodo = int32(newIndex)
	// Entrada ".." apunta al directorio padre.
	copy(folder.B_content[1].B_name[:], "..")
	folder.B_content[1].B_inodo = int32(parentIndex)
	// Las demás entradas se dejan vacías.
	// La escritura se realizará luego mediante MultiBlockUpdateFile.
	fmt.Printf("InitializeFolder: '.' = %d, '..' = %d\n", newIndex, parentIndex)
	return nil
}

func allocateBlock(file *os.File, sb Structs.Superblock) (int32, error) {
	blockCount := sb.S_blocks_count
	bmOffset := int64(sb.S_bm_block_start)
	for i := int32(0); i < blockCount; i++ {
		var bit byte
		if err := Utilities.ReadObject(file, &bit, bmOffset+int64(i)); err != nil {
			return -1, err
		}
		if bit == 0 {
			if err := Utilities.WriteObject(file, byte(1), bmOffset+int64(i)); err != nil {
				return -1, err
			}
			return i, nil
		}
	}
	return -1, fmt.Errorf("no se encontró bloque libre")
}

// freeBlockInBitmap marca un bloque como libre en el bitmap.
func freeBlockInBitmap(file *os.File, sb Structs.Superblock, blockIndex int32) error {
	bmOffset := int64(sb.S_bm_block_start) + int64(blockIndex)
	return Utilities.WriteObject(file, byte(0), bmOffset)
}

func GetInodeFromPath(path string, file *os.File, sb Structs.Superblock) (*Structs.Inode, int64) {
	if path == "/" {
		return GetInodeFromPathByIndex(0, file, sb)
	}
	components := strings.Split(path, "/")[1:]
	currentIndex := 0
	var _ int64
	for _, comp := range components {
		inode, _ := GetInodeFromPathByIndex(currentIndex, file, sb)
		folder, err := ReadFolderBlock(file, sb, inode.I_block[0])
		if err != nil {
			return nil, 0
		}
		found := false
		for _, entry := range folder.B_content {
			if strings.Trim(string(entry.B_name[:]), "\x00") == comp {
				currentIndex = int(entry.B_inodo)
				found = true
				break
			}
		}
		if !found {
			return nil, 0
		}
	}
	return GetInodeFromPathByIndex(currentIndex, file, sb)
}

func hasWritePermission(inode Structs.Inode, currentUser string) bool {
	// Suponiendo que el usuario root tiene permiso total.
	if currentUser == "root" {
		return true
	}
	permStr := strings.Trim(string(inode.I_perm[:]), "\x00")
	if len(permStr) < 1 {
		return false
	}
	// En notación numérica, el permiso de escritura es 2 (ejemplo: 6 => 4+2, 7 => 4+2+1)
	ownerPerm, err := strconv.Atoi(string(permStr[0]))
	if err != nil {
		return false
	}
	return ownerPerm%4 >= 2 // simplificación: si el dígito (4+2) es mayor o igual a 6, tiene escritura.
}

func hasReadPermission(inode Structs.Inode, currentUser string) bool {
	if currentUser == "root" {
		return true
	}
	permStr := strings.Trim(string(inode.I_perm[:]), "\x00")
	if len(permStr) < 1 {
		return false
	}
	// Convertir el primer dígito a número (por ejemplo, '7' => 7, '6' => 6, etc.)
	ownerPerm, err := strconv.Atoi(string(permStr[0]))
	if err != nil {
		return false
	}
	// En sistemas Unix, un permiso numérico mayor o igual a 4 implica que el propietario puede leer.
	return ownerPerm >= 4
}

func GetCurrentSessionPartition() *DiskManagement.MountedPartition {
	mounted := DiskManagement.GetMountedPartitions()
	for disk := range mounted {
		for i := range mounted[disk] {
			if mounted[disk][i].LoggedIn {
				return &mounted[disk][i]
			}
		}
	}
	return nil
}

func Login(user string, pass string, id string) error {
	OutPut.Println("======Start LOGIN======")
	fmt.Printf("User: %s, Pass: %s, ID: %s\n", user, pass, id)
	id = strings.ToUpper(id)
	if user == "" {
		return fmt.Errorf("user cannot be empty")
	}
	if pass == "" {
		return fmt.Errorf("password cannot be empty")
	}
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}

	if currentUser.loggedIn {
		return fmt.Errorf("another user is already logged in")
	}

	partition, diskPath, err := stores.GetMountedPartition(id)
	if err != nil {
		return fmt.Errorf("error finding partition: %v", err)
	}

	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	var superblock Structs.Superblock
	if err := Utilities.ReadObject(file, &superblock, int64(partition.Start)); err != nil {
		return fmt.Errorf("error reading superblock: %v", err)
	}

	// Find users.txt inode
	var rootInode Structs.Inode
	if err := Utilities.ReadObject(file, &rootInode, int64(superblock.S_inode_start)); err != nil {
		return fmt.Errorf("error reading root inode: %v", err)
	}

	var usersInodeIndex int32 = -1
	for i := 0; i < 15; i++ {
		if rootInode.I_block[i] != -1 {
			var block Structs.Folderblock
			if err := Utilities.ReadObject(file, &block, int64(superblock.S_block_start+rootInode.I_block[i]*superblock.S_block_size)); err != nil {
				continue
			}
			for _, entry := range block.B_content {
				if strings.Trim(string(entry.B_name[:]), "\x00") == "users.txt" && entry.B_inodo != -1 {
					usersInodeIndex = entry.B_inodo
					break
				}
			}
			if usersInodeIndex != -1 {
				break
			}
		}
	}
	if usersInodeIndex == -1 {
		return fmt.Errorf("users.txt not found")
	}

	var usersInode Structs.Inode
	if err := Utilities.ReadObject(file, &usersInode, int64(superblock.S_inode_start+usersInodeIndex*superblock.S_inode_size)); err != nil {
		return fmt.Errorf("error reading users.txt inode: %v", err)
	}

	var usersBlock Structs.Fileblock
	if err := Utilities.ReadObject(file, &usersBlock, int64(superblock.S_block_start+usersInode.I_block[0]*superblock.S_block_size)); err != nil {
		return fmt.Errorf("error reading users.txt block: %v", err)
	}

	content := strings.Trim(string(usersBlock.B_content[:]), "\x00")
	fmt.Printf("Users.txt content: %q\n", content)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		fields := strings.Split(line, ",")
		if len(fields) == 5 && fields[1] == "U" && strings.EqualFold(fields[3], user) && fields[4] == pass {
			currentUser.loggedIn = true
			currentUser.user = user
			currentUser.partition = id
			mounted := DiskManagement.GetMountedPartitions() //codigo ya corregido
			for disk := range mounted {
				for i := range mounted[disk] {
					if mounted[disk][i].ID == id {
						mounted[disk][i].LoggedIn = true
						break
					}
				}
			} //fin

			OutPut.Println("Login successful")
			OutPut.Println("======End LOGIN======")
			return nil
		}
	}

	return fmt.Errorf("invalid user or password")
}

func Logout() error {
	OutPut.Println("======Start LOGOUT======")

	if !currentUser.loggedIn {
		OutPut.Println("Error: No hay sesión activa")
		OutPut.Println("======End LOGOUT======")
		return fmt.Errorf("no hay sesión activa")
	}

	// Buscar y actualizar la partición con sesión iniciada
	for disk := range DiskManagement.GetMountedPartitions() {
		for i := range DiskManagement.GetMountedPartitions()[disk] {
			if DiskManagement.GetMountedPartitions()[disk][i].ID == currentUser.partition {
				DiskManagement.GetMountedPartitions()[disk][i].LoggedIn = false
				break
			}
		}
	}

	fmt.Printf("Sesión finalizada para el usuario %s en la partición %s\n", currentUser.user, currentUser.partition)
	currentUser.loggedIn = false
	currentUser.user = ""
	currentUser.partition = ""

	OutPut.Println("======End LOGOUT======")
	return nil
}

func InitSearch(path string, file *os.File, sb Structs.Superblock) int32 {
	// Simplified search for /users.txt in the root directory
	var rootInode Structs.Inode
	if err := Utilities.ReadObject(file, &rootInode, int64(sb.S_inode_start)); err != nil {
		fmt.Printf("Error reading root inode: %v\n", err)
		return -1
	}

	for i := 0; i < 15; i++ {
		if rootInode.I_block[i] == -1 {
			continue
		}
		var block Structs.Fileblock
		if err := Utilities.ReadObject(file, &block, int64(sb.S_block_start+rootInode.I_block[i]*sb.S_block_size)); err != nil {
			fmt.Printf("Error reading block %d: %v\n", i, err)
			continue
		}
		content := strings.Trim(string(block.B_content[:]), "\x00")
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if strings.Contains(line, ",U,") || strings.Contains(line, ",G,") {
				continue // Skip user/group entries
			}
			if strings.Contains(line, "users.txt") {
				// Assume users.txt is in the root block
				return 1 // Inode 1 is typically users.txt
			}
		}
	}
	return -1
}

func GetInodeFileData(inode Structs.Inode, file *os.File, sb Structs.Superblock) string {
	var data strings.Builder
	for i := 0; i < 15; i++ {
		if inode.I_block[i] == -1 {
			continue
		}
		var block Structs.Fileblock
		if err := Utilities.ReadObject(file, &block, int64(sb.S_block_start+inode.I_block[i]*sb.S_block_size)); err != nil {
			fmt.Printf("Error reading block %d: %v\n", inode.I_block[i], err)
			continue
		}
		content := strings.Trim(string(block.B_content[:]), "\x00")
		data.WriteString(content)
	}
	return data.String()
}

func MultiBlockUpdate(inode *Structs.Inode, content string, file *os.File, sb Structs.Superblock, inodeOffset int64, partitionStart int64) error {
	// Update the first block of users.txt
	if inode.I_block[0] == -1 {
		inode.I_block[0] = sb.S_first_blo
		sb.S_first_blo++
		sb.S_free_blocks_count--
	}
	var block Structs.Fileblock
	copy(block.B_content[:], content)
	if err := Utilities.WriteObject(file, block, int64(sb.S_block_start+inode.I_block[0]*sb.S_block_size)); err != nil {
		return fmt.Errorf("error writing block: %v", err)
	}

	// Update inode
	inode.I_size = int32(len(content))
	copy(inode.I_mtime[:], time.Now().Format("2006-01-02 15:04:05"))
	if err := Utilities.WriteObject(file, *inode, inodeOffset); err != nil {
		return fmt.Errorf("error writing inode: %v", err)
	}

	// Update superblock
	if err := Utilities.WriteObject(file, sb, partitionStart); err != nil {
		return fmt.Errorf("error writing superblock: %v", err)
	}
	return nil
}

func Mkgrp(name string) error {
	OutPut.Println("======Start MKGRP======")
	fmt.Printf("Group Name: %s\n", name)

	if !currentUser.loggedIn {
		return fmt.Errorf("necesita iniciar sesión")
	}
	if currentUser.user != "root" {
		return fmt.Errorf("solo el usuario root puede ejecutar mkgrp")
	}

	partition, diskPath, err := stores.GetMountedPartition(currentUser.partition)
	if err != nil {
		return fmt.Errorf("error finding partition %s: %v", currentUser.partition, err)
	}

	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return fmt.Errorf("error opening file %s: %v", diskPath, err)
	}
	defer file.Close()

	var sb Structs.Superblock
	if err := Utilities.ReadObject(file, &sb, int64(partition.Start)); err != nil {
		return fmt.Errorf("error reading superblock: %v", err)
	}

	indexInode := InitSearch("/users.txt", file, sb)
	if indexInode < 0 {
		return fmt.Errorf("no se encontró el archivo users.txt")
	}

	inodeOffset := int64(sb.S_inode_start) + int64(indexInode)*int64(binary.Size(Structs.Inode{}))
	var usersInode Structs.Inode
	if err := Utilities.ReadObject(file, &usersInode, inodeOffset); err != nil {
		return fmt.Errorf("error reading users.txt inode: %v", err)
	}

	data := GetInodeFileData(usersInode, file, sb)
	trimmedData := strings.TrimRight(data, "\n")
	lines := strings.Split(trimmedData, "\n")

	for _, line := range lines {
		tokens := strings.Split(line, ",")
		if len(tokens) >= 3 && strings.TrimSpace(tokens[1]) == "G" && strings.TrimSpace(tokens[2]) == name {
			return fmt.Errorf("el grupo %s ya existe", name)
		}
	}

	newGroupID := 2
	for _, line := range lines {
		tokens := strings.Split(line, ",")
		if len(tokens) >= 3 && strings.TrimSpace(tokens[1]) == "G" {
			idInt, _ := strconv.Atoi(strings.TrimSpace(tokens[0]))
			if idInt >= newGroupID {
				newGroupID = idInt + 1
			}
		}
	}

	newRecord := fmt.Sprintf("%d,G,%s\n", newGroupID, name)
	newContent := trimmedData + "\n" + newRecord

	if err := MultiBlockUpdate(&usersInode, newContent, file, sb, inodeOffset, int64(partition.Start)); err != nil {
		return fmt.Errorf("error updating users.txt: %v", err)
	}

	fmt.Printf("Grupo %s creado exitosamente con ID %d\n", name, newGroupID)
	OutPut.Println("======End MKGRP======")
	return nil
}

func initInode(inode *Structs.Inode, date string) {
	*inode = Structs.Inode{}
	inode.I_uid = 1
	inode.I_gid = 1
	inode.I_size = 0
	copy(inode.I_ctime[:], date)
	copy(inode.I_mtime[:], date)
	copy(inode.I_type[:], "0") // 0 for directory, 1 for file
	copy(inode.I_perm[:], "664")
	for i := 0; i < 15; i++ {
		inode.I_block[i] = -1
	}
}

func CreateRootAndUsersFile(newSuperblock Structs.Superblock, date string, file *os.File) error {
	var Inode0, Inode1 Structs.Inode
	initInode(&Inode0, date)
	initInode(&Inode1, date)

	// Set Inode0 as directory (root) and Inode1 as file (users.txt)
	copy(Inode0.I_type[:], "0") // Directory
	copy(Inode1.I_type[:], "1") // File
	Inode0.I_block[0] = 0       // Root directory block
	Inode1.I_block[0] = 1       // Users.txt block

	// Set size for users.txt
	data := "1,G,root\n1,U,root,root,123\n"
	Inode1.I_size = int32(len(data))

	var Folderblock0 Structs.Folderblock
	Folderblock0.B_content[0].B_inodo = 0
	copy(Folderblock0.B_content[0].B_name[:], ".")
	Folderblock0.B_content[1].B_inodo = 0
	copy(Folderblock0.B_content[1].B_name[:], "..")
	Folderblock0.B_content[2].B_inodo = 1
	copy(Folderblock0.B_content[2].B_name[:], "users.txt\000") // Pad with null byte

	var Fileblock1 Structs.Fileblock
	copy(Fileblock1.B_content[:], data)

	// Write inodes and blocks
	if err := Utilities.WriteObject(file, Inode0, int64(newSuperblock.S_inode_start)); err != nil {
		return err
	}
	if err := Utilities.WriteObject(file, Inode1, int64(newSuperblock.S_inode_start+int32(binary.Size(Structs.Inode{})))); err != nil {
		return err
	}
	if err := Utilities.WriteObject(file, Folderblock0, int64(newSuperblock.S_block_start)); err != nil {
		return err
	}
	if err := Utilities.WriteObject(file, Fileblock1, int64(newSuperblock.S_block_start+int32(binary.Size(Structs.Folderblock{})))); err != nil {
		return err
	}

	// Update superblock
	newSuperblock.S_fist_ino = 2  // Next available inode
	newSuperblock.S_first_blo = 2 // Next available block
	newSuperblock.S_free_inodes_count--
	newSuperblock.S_free_blocks_count--
	if err := Utilities.WriteObject(file, newSuperblock, int64(newSuperblock.S_bm_inode_start-newSuperblock.S_inode_size)); err != nil {
		return err
	}

	return nil
}

// getCurrentSessionPartition retorna la primera partición montada que tenga sesión activa.
func Rmgrp(name string) error {
	OutPut.Println("======Start RMGRP======")
	fmt.Printf("Group Name: %s\n", name)

	if !currentUser.loggedIn {
		return fmt.Errorf("necesita iniciar sesión")
	}
	if currentUser.user != "root" {
		return fmt.Errorf("solo el usuario root puede ejecutar rmgrp")
	}

	partition, diskPath, err := stores.GetMountedPartition(currentUser.partition)
	if err != nil {
		return fmt.Errorf("error encontrando la partición: %v", err)
	}

	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return fmt.Errorf("error abriendo el archivo: %v", err)
	}
	defer file.Close()

	var sb Structs.Superblock
	if err := Utilities.ReadObject(file, &sb, int64(partition.Start)); err != nil {
		return fmt.Errorf("error leyendo el Superblock: %v", err)
	}

	indexInode := InitSearch("/users.txt", file, sb)
	if indexInode < 0 {
		return fmt.Errorf("no se encontró el archivo users.txt")
	}
	inodeOffset := int64(sb.S_inode_start) + int64(indexInode)*int64(binary.Size(Structs.Inode{}))

	var usersInode Structs.Inode
	if err := Utilities.ReadObject(file, &usersInode, inodeOffset); err != nil {
		return fmt.Errorf("error leyendo el inodo de users.txt: %v", err)
	}

	data := GetInodeFileData(usersInode, file, sb)
	lines := strings.Split(data, "\n")
	found := false

	for i, line := range lines {
		tokens := strings.Split(strings.TrimSpace(line), ",")
		if len(tokens) >= 3 && strings.TrimSpace(tokens[1]) == "G" && strings.TrimSpace(tokens[2]) == name {
			if strings.TrimSpace(tokens[0]) == "0" {
				return fmt.Errorf("el grupo ya fue eliminado")
			}
			tokens[0] = "0"
			lines[i] = strings.Join(tokens, ",")
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("el grupo no existe")
	}

	newContent := strings.Join(lines, "\n")
	if err := MultiBlockUpdate(&usersInode, newContent, file, sb, inodeOffset, int64(partition.Start)); err != nil {

		return fmt.Errorf("error actualizando users.txt: %v", err)
	}

	OutPut.Println("Grupo eliminado exitosamente")
	OutPut.Println("======End RMGRP======")
	return nil
}

func Mkusr(user, pass, grp string) error {
	OutPut.Println("======Start MKUSR======")
	fmt.Printf("User: %s, Pass: %s, Group: %s\n", user, pass, grp)

	if !currentUser.loggedIn {
		return fmt.Errorf("necesita iniciar sesión")
	}
	if currentUser.user != "root" {
		return fmt.Errorf("solo el usuario root puede ejecutar mkusr")
	}

	if len(user) > 10 || len(pass) > 10 || len(grp) > 10 {
		return fmt.Errorf("user, pass o group exceden el máximo de 10 caracteres")
	}

	partition, diskPath, err := stores.GetMountedPartition(currentUser.partition)
	if err != nil {
		return fmt.Errorf("error encontrando la partición: %v", err)
	}

	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return fmt.Errorf("error abriendo el archivo: %v", err)
	}
	defer file.Close()

	var sb Structs.Superblock
	if err := Utilities.ReadObject(file, &sb, int64(partition.Start)); err != nil {
		return fmt.Errorf("error leyendo el Superblock: %v", err)
	}

	indexInode := InitSearch("/users.txt", file, sb)
	if indexInode < 0 {
		return fmt.Errorf("no se encontró el archivo users.txt")
	}
	inodeOffset := int64(sb.S_inode_start) + int64(indexInode)*int64(binary.Size(Structs.Inode{}))

	var usersInode Structs.Inode
	if err := Utilities.ReadObject(file, &usersInode, inodeOffset); err != nil {
		return fmt.Errorf("error leyendo el inodo de users.txt: %v", err)
	}

	data := GetInodeFileData(usersInode, file, sb)
	lines := strings.Split(strings.TrimRight(data, "\n"), "\n")

	// Validar si ya existe el usuario
	for _, line := range lines {
		tokens := strings.Split(strings.TrimSpace(line), ",")
		if len(tokens) >= 5 && strings.TrimSpace(tokens[1]) == "U" {
			if strings.TrimSpace(tokens[3]) == user {
				return fmt.Errorf("el usuario ya existe")
			}
		}
	}

	// Validar si el grupo existe y no está eliminado
	groupExists := false
	for _, line := range lines {
		tokens := strings.Split(strings.TrimSpace(line), ",")
		if len(tokens) >= 3 && strings.TrimSpace(tokens[1]) == "G" && strings.TrimSpace(tokens[0]) != "0" {
			if strings.TrimSpace(tokens[2]) == grp {
				groupExists = true
				break
			}
		}
	}
	if !groupExists {
		return fmt.Errorf("el grupo no existe o está eliminado")
	}

	// Calcular nuevo ID de usuario
	newUserID := 2
	for _, line := range lines {
		tokens := strings.Split(strings.TrimSpace(line), ",")
		if len(tokens) >= 1 {
			id := strings.TrimSpace(tokens[0])
			if val, err := strconv.Atoi(id); err == nil && val >= newUserID {
				newUserID = val + 1
			}
		}
	}

	newRecord := fmt.Sprintf("%d,U,%s,%s,%s\n", newUserID, grp, user, pass)
	newContent := strings.Join(lines, "\n") + "\n" + newRecord

	if err := MultiBlockUpdate(&usersInode, newContent, file, sb, inodeOffset, int64(partition.Start)); err != nil {
		return fmt.Errorf("error actualizando users.txt: %v", err)
	}

	OutPut.Println("Usuario creado exitosamente.")
	OutPut.Println("======End MKUSR======")
	return nil
}

func Rmusr(username string) error {
	OutPut.Println("======Start RMUSR======")
	fmt.Printf("Usuario a eliminar: %s\n", username)

	if !currentUser.loggedIn {
		return fmt.Errorf("necesita iniciar sesión")
	}
	if currentUser.user != "root" {
		return fmt.Errorf("solo el usuario root puede ejecutar rmusr")
	}

	if len(username) > 10 {
		return fmt.Errorf("el nombre de usuario excede los 10 caracteres")
	}

	partition, diskPath, err := stores.GetMountedPartition(currentUser.partition)
	if err != nil {
		return fmt.Errorf("error encontrando la partición: %v", err)
	}

	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return fmt.Errorf("error abriendo el archivo del disco: %v", err)
	}
	defer file.Close()

	var sb Structs.Superblock
	if err := Utilities.ReadObject(file, &sb, int64(partition.Start)); err != nil {
		return fmt.Errorf("error leyendo el Superblock: %v", err)
	}

	indexInode := InitSearch("/users.txt", file, sb)
	if indexInode < 0 {
		return fmt.Errorf("no se encontró el archivo users.txt")
	}
	inodeOffset := int64(sb.S_inode_start) + int64(indexInode)*int64(binary.Size(Structs.Inode{}))

	var usersInode Structs.Inode
	if err := Utilities.ReadObject(file, &usersInode, inodeOffset); err != nil {
		return fmt.Errorf("error leyendo el inodo de users.txt: %v", err)
	}

	data := GetInodeFileData(usersInode, file, sb)
	lines := strings.Split(data, "\n")
	found := false

	for i, line := range lines {
		tokens := strings.Split(strings.TrimSpace(line), ",")
		if len(tokens) >= 5 && strings.TrimSpace(tokens[1]) == "U" && strings.TrimSpace(tokens[3]) == username {
			if strings.TrimSpace(tokens[0]) == "0" {
				return fmt.Errorf("el usuario ya está eliminado")
			}
			tokens[0] = "0"
			lines[i] = strings.Join(tokens, ",")
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("el usuario no existe")
	}

	newContent := strings.Join(lines, "\n")
	if err := MultiBlockUpdate(&usersInode, newContent, file, sb, inodeOffset, int64(partition.Start)); err != nil {
		return fmt.Errorf("error actualizando users.txt: %v", err)
	}

	OutPut.Println("Usuario eliminado correctamente")
	OutPut.Println("======End RMUSR======")
	return nil
}

func Mkfs(id string, type_ string, fs string) error {
	OutPut.Println("======Start MKFS======")
	OutPut.Println("ID:", id, "Type:", type_, "FS:", fs)

	if type_ != "FULL" {
		return fmt.Errorf("type must be FULL")
	}
	if fs != "2FS" && fs != "3FS" {
		fs = "2FS"
	}

	partition, diskPath, err := stores.GetMountedPartition(id)
	if err != nil {
		return fmt.Errorf("error retrieving mounted partition: %v", err)
	}
	if partition == nil {
		return fmt.Errorf("partition with ID %s not found", id)
	}
	fmt.Printf("Partition found: Name=%s, Size=%d, Start=%d\n", strings.Trim(string(partition.Name[:]), "\x00"), partition.Size, partition.Start)

	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return fmt.Errorf("error opening file %s: %v", diskPath, err)
	}
	defer file.Close()

	// Create superblock
	var superblock Structs.Superblock
	minSize := int64(binary.Size(Structs.Superblock{})) + 4 + int64(binary.Size(Structs.Inode{})) + 3*int64(binary.Size(Structs.Fileblock{}))

	if fs == "3FS" {
		minSize += int64(binary.Size(Structs.Journaling{}))
	}
	if int64(partition.Size) < minSize {
		return fmt.Errorf("partition size too small to format: Size=%d bytes, Minimum required=%d bytes", partition.Size, minSize)
	}

	n := int32((int32(partition.Size) - int32(binary.Size(Structs.Superblock{}))) / (4 + int32(binary.Size(Structs.Inode{})) + 3*int32(binary.Size(Structs.Fileblock{}))))
	if fs == "3FS" {
		n = int32((int32(partition.Size) - int32(binary.Size(Structs.Superblock{})) - int32(binary.Size(Structs.Journaling{}))) / (4 + int32(binary.Size(Structs.Inode{})) + 3*int32(binary.Size(Structs.Fileblock{}))))
	}
	//n = n / 4
	if n <= 0 {
		return fmt.Errorf("partition size too small to create inodes: Size=%d bytes, Minimum required=%d bytes", partition.Size, minSize*4)
	}
	fmt.Printf("Calculated inodes: n=%d\n", n)

	superblock.S_filesystem_type = 2
	if fs == "3FS" {
		superblock.S_filesystem_type = 3
	}
	superblock.S_inodes_count = n
	superblock.S_blocks_count = 3 * n
	superblock.S_free_blocks_count = 3 * n
	superblock.S_free_inodes_count = n
	superblock.S_mnt_count = 1
	superblock.S_magic = 0xEF53
	superblock.S_inode_size = int32(binary.Size(Structs.Inode{}))
	superblock.S_block_size = int32(binary.Size(Structs.Fileblock{}))
	copy(superblock.S_mtime[:], time.Now().Format("2006-01-02 15:04:05"))
	copy(superblock.S_umtime[:], time.Now().Format("2006-01-02 15:04:05"))
	superblock.S_bm_inode_start = int32(partition.Start + int64(binary.Size(Structs.Superblock{})))
	if fs == "3FS" {
		superblock.S_bm_inode_start += int32(binary.Size(Structs.Journaling{}))
	}
	superblock.S_bm_block_start = superblock.S_bm_inode_start + n
	superblock.S_inode_start = superblock.S_bm_block_start + 3*n
	superblock.S_block_start = superblock.S_inode_start + n*superblock.S_inode_size
	superblock.S_fist_ino = 0
	superblock.S_first_blo = 0

	if err := Utilities.WriteObject(file, superblock, int64(partition.Start)); err != nil {
		return fmt.Errorf("error writing superblock: %v", err)
	}

	// Write bitmaps
	bitmapInodes := make([]byte, n)
	bitmapBlocks := make([]byte, 3*n)
	if err := Utilities.WriteObject(file, bitmapInodes, int64(superblock.S_bm_inode_start)); err != nil {
		return fmt.Errorf("error writing inode bitmap: %v", err)
	}
	if err := Utilities.WriteObject(file, bitmapBlocks, int64(superblock.S_bm_block_start)); err != nil {
		return fmt.Errorf("error writing block bitmap: %v", err)
	}

	// Initialize inodes and blocks
	for i := int32(0); i < n; i++ {
		var inode Structs.Inode
		inode.I_block = [15]int32{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
		if err := Utilities.WriteObject(file, inode, int64(superblock.S_inode_start+i*superblock.S_inode_size)); err != nil {
			return fmt.Errorf("error writing inode %d: %v", i, err)
		}
	}
	for i := int32(0); i < 3*n; i++ {
		var block Structs.Fileblock
		if err := Utilities.WriteObject(file, block, int64(superblock.S_block_start+i*superblock.S_block_size)); err != nil {
			return fmt.Errorf("error writing block %d: %v", i, err)
		}
	}

	// Crea root directory y users.txt
	if err := CreateRootAndUsersFile(superblock, time.Now().Format("2006-01-02 15:04:05"), file); err != nil {
		return fmt.Errorf("error creating root and users file: %v", err)
	}

	OutPut.Println("Partition formatted successfully")
	OutPut.Println("======End MKFS======")
	return nil
}

func Mkfile(path string, createParents bool, size int, cont string) string {
	// Verificar sesión.
	currentPartition := GetCurrentSessionPartition()
	if currentPartition == nil {
		return "Error: Necesita iniciar sesión"
	}

	path = normalizePath(path)

	lastSlash := strings.LastIndex(path, "/")
	if lastSlash < 0 {
		return "Error: Ruta inválida"
	}
	parentPath := path[:lastSlash]
	fileName := path[lastSlash+1:]
	if fileName == "" {
		return "Error: No se especificó el nombre del archivo"
	}

	// Buscar o crear la carpeta padre.
	parentIndex := SearchPath(parentPath, createParents, currentPartition)
	if parentIndex < 0 {
		return fmt.Sprintf("Error: La carpeta padre '%s' no existe", parentPath)
	}

	// Abrir el disco.
	diskFile, err := Utilities.OpenFile(currentPartition.Path)
	if err != nil {
		return fmt.Sprintf("Error: No se pudo abrir el disco: %v", err)
	}
	defer diskFile.Close()

	// Leer MBR y Superblock.
	var TempMBR Structs.MRB
	if err := Utilities.ReadObject(diskFile, &TempMBR, 0); err != nil {
		return fmt.Sprintf("Error: No se pudo leer el MBR: %v", err)
	}
	var partIdx int = -1
	for i := 0; i < 4; i++ {
		if strings.Contains(string(TempMBR.Partitions[i].Id[:]), currentPartition.ID) {
			partIdx = i
			break
		}
	}
	if partIdx == -1 {
		return "Error: Partición no encontrada"
	}
	var sbSuper Structs.Superblock
	if err := Utilities.ReadObject(diskFile, &sbSuper, int64(TempMBR.Partitions[partIdx].Start)); err != nil {
		return fmt.Sprintf("Error al leer el Superblock: %v", err)
	}

	// Verificar permiso de escritura en la carpeta padre.
	parentInode, _ := GetInodeFromPath(parentPath, diskFile, sbSuper)
	if parentInode == nil {
		return fmt.Sprintf("Error: La carpeta padre '%s' no existe", parentPath)
	}
	if !hasWritePermission(*parentInode, currentUser.user) {
		return "Error: No tiene permiso de escritura en la carpeta padre"
	}

	// Verificar si el archivo ya existe en la carpeta padre.
	if EntryExistsInFolder(*parentInode, diskFile, sbSuper, fileName) {
		return "Error: El archivo ya existe. No se permite sobreescribir."
	}

	// Determinar el contenido a escribir.
	var fileContent string
	if strings.TrimSpace(cont) != "" {
		bytes, err := os.ReadFile(cont)
		if err != nil {
			return fmt.Sprintf("Error: No se pudo leer el archivo de contenido (%s): %v", cont, err)
		}
		fileContent = string(bytes)
	} else if size > 0 {
		var sbuilder strings.Builder
		digits := "0123456789"
		for sbuilder.Len() < size {
			sbuilder.WriteString(digits)
		}
		fileContent = sbuilder.String()[:size]
	} else {
		fileContent = ""
	}

	perm := "664"             // permisos por defecto
	owner := currentUser.user // Puedes buscar el propietario real si lo necesitas
	group := "default"        // Puedes buscar el grupo real si lo necesitas

	// Asignar un nuevo inodo para el archivo.
	newFileInode, newInodeOffset, newFileIndex, err := allocateInode(diskFile, sbSuper, owner, group, perm, false)
	if err != nil {
		return fmt.Sprintf("Error al asignar un nuevo inodo: %v", err)
	}
	fmt.Printf("MKFILE: Nuevo inodo asignado: índice %d, offset %d\n", newFileIndex, newInodeOffset)

	// Escribir el contenido en múltiples bloques, usando apuntadores directos e indirectos.
	if err := MultiBlockUpdateFile(newFileInode, fileContent, diskFile, sbSuper, newInodeOffset); err != nil {
		return fmt.Sprintf("Error al escribir el archivo: %v", err)
	}

	// Agregar una entrada en la carpeta padre.
	if err := AddEntryToFolderByIndex(parentIndex, diskFile, sbSuper, fileName, newFileIndex); err != nil {
		return fmt.Sprintf("Error al agregar la entrada en la carpeta padre: %v", err)
	}

	return "Archivo creado con éxito"
}

func Cat(params map[string]string) string {
	// Verificar que exista una sesión activa.
	currentPartition := GetCurrentSessionPartition()
	if currentPartition == nil {
		return "Error: Necesita iniciar sesión"
	}
	if currentUser.user == "" {
		return "Error: No se encontró un usuario logueado"
	}

	// Abrir el disco de la partición activa.
	file, err := Utilities.OpenFile(currentPartition.Path)
	if err != nil {
		return fmt.Sprintf("Error: No se pudo abrir el archivo: %v", err)
	}
	defer file.Close()

	// Leer el MBR y localizar la partición activa.
	var TempMBR Structs.MRB
	if err := Utilities.ReadObject(file, &TempMBR, 0); err != nil {
		return fmt.Sprintf("Error: No se pudo leer el MBR: %v", err)
	}
	var partIndex int = -1
	for i := 0; i < 4; i++ {
		if strings.Contains(string(TempMBR.Partitions[i].Id[:]), currentPartition.ID) {
			partIndex = i
			break
		}
	}
	if partIndex == -1 {
		return "Error: Partición no encontrada"
	}

	// Leer el Superblock.
	var sb Structs.Superblock
	if err := Utilities.ReadObject(file, &sb, int64(TempMBR.Partitions[partIndex].Start)); err != nil {
		return fmt.Sprintf("Error al leer el Superblock: %v", err)
	}

	// Ordenar las claves de los parámetros (file1, file2, etc.) en orden ascendente.
	keys := []int{}
	for k := range params {
		// Se espera que k tenga el formato "fileN"
		if strings.HasPrefix(k, "file") {
			numStr := strings.TrimPrefix(k, "file")
			if num, err := strconv.Atoi(numStr); err == nil {
				keys = append(keys, num)
			}
		}
	}
	sort.Ints(keys)

	// Acumular el contenido de cada archivo.
	var outputBuilder strings.Builder

	// Iterar sobre cada archivo en el orden especificado.
	for _, num := range keys {
		key := "file" + strconv.Itoa(num)
		path := params[key]
		if path == "" {
			continue
		}

		// Buscar el inodo del archivo usando InitSearch.
		indexInode := InitSearch(path, file, sb)
		if indexInode < 0 {
			return fmt.Sprintf("Error: El archivo %s no existe", path)
		}

		// Calcular el offset del inodo.
		inodeOffset := int64(sb.S_inode_start) + int64(indexInode)*int64(binary.Size(Structs.Inode{}))
		var fileInode Structs.Inode
		if err := Utilities.ReadObject(file, &fileInode, inodeOffset); err != nil {
			return fmt.Sprintf("Error al leer el inodo del archivo %s: %v", path, err)
		}

		// Verificar permiso de lectura.
		if !hasReadPermission(fileInode, currentUser.user) {
			return fmt.Sprintf("Error: No tiene permiso de lectura para el archivo %s", path)
		}

		// Obtener el contenido del archivo.
		data := GetInodeFileData(fileInode, file, sb)
		outputBuilder.WriteString(data)
		outputBuilder.WriteString("\n")
	}

	return outputBuilder.String()
}

func Mkdir(path string, createParents bool) string {
	currentPartition := GetCurrentSessionPartition()
	if currentPartition == nil {
		return "Error: Necesita iniciar sesión"
	}
	if !strings.HasPrefix(path, "/") {
		return "Error: La ruta debe comenzar con '/'"
	}
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash < 0 {
		return "Error: Ruta inválida"
	}
	parentPath := path[:lastSlash]
	folderName := path[lastSlash+1:]
	if folderName == "" {
		return "Error: No se especificó el nombre de la carpeta"
	}

	// Buscar o crear la carpeta padre.
	parentIndex := SearchPath(parentPath, createParents, currentPartition)
	if parentIndex < 0 {
		return fmt.Sprintf("Error: La carpeta padre '%s' no existe", parentPath)
	}

	// Abrir el disco.
	diskFile, err := Utilities.OpenFile(currentPartition.Path)
	if err != nil {
		return fmt.Sprintf("Error: No se pudo abrir el disco: %v", err)
	}
	defer diskFile.Close()

	// Leer MBR y Superblock.
	var TempMBR Structs.MRB
	if err := Utilities.ReadObject(diskFile, &TempMBR, 0); err != nil {
		return fmt.Sprintf("Error: No se pudo leer el MBR: %v", err)
	}
	var partIdx int = -1
	for i := 0; i < 4; i++ {
		if strings.Contains(string(TempMBR.Partitions[i].Id[:]), currentPartition.ID) {
			partIdx = i
			break
		}
	}
	if partIdx == -1 {
		return "Error: Partición no encontrada"
	}
	var sbSuper Structs.Superblock
	if err := Utilities.ReadObject(diskFile, &sbSuper, int64(TempMBR.Partitions[partIdx].Start)); err != nil {
		return fmt.Sprintf("Error al leer el Superblock: %v", err)
	}

	// Verificar que la carpeta padre exista y tenga permiso de escritura.
	parentInode, _ := GetInodeFromPath(parentPath, diskFile, sbSuper)
	if parentInode == nil {
		return fmt.Sprintf("Error: La carpeta padre '%s' no existe", parentPath)
	}
	if !hasWritePermission(*parentInode, currentUser.user) {
		return "Error: No tiene permiso de escritura en la carpeta padre"
	}

	// Verificar si la carpeta ya existe en la carpeta padre.
	if EntryExistsInFolder(*parentInode, diskFile, sbSuper, folderName) {
		return "Error: La carpeta ya existe"
	}

	// Asignar un nuevo inodo para la carpeta (indicando que es directorio).
	newFolderInode, newInodeOffset, newFolderIndex, err := allocateInode(diskFile, sbSuper, currentUser.user, "default", "664", true)
	if err != nil {
		return fmt.Sprintf("Error al asignar un nuevo inodo: %v", err)
	}
	fmt.Printf("MKDIR: Nuevo inodo asignado: índice %d, offset %d\n", newFolderIndex, newInodeOffset)

	// Inicializar la carpeta con entradas "." y "..". Se usa el índice del padre (parentIndex).
	if err := InitializeFolder(newFolderInode, parentInode, newFolderIndex, parentIndex); err != nil {
		return fmt.Sprintf("Error al inicializar la carpeta: %v", err)
	}

	// Escribir el contenido inicial (vacío) en la carpeta.
	if err := MultiBlockUpdateFile(newFolderInode, "", diskFile, sbSuper, newInodeOffset); err != nil {
		return fmt.Sprintf("Error al escribir la carpeta: %v", err)
	}

	// Agregar una entrada en la carpeta padre.
	if err := AddEntryToFolderByIndex(parentIndex, diskFile, sbSuper, folderName, newFolderIndex); err != nil {
		return fmt.Sprintf("Error al agregar la entrada en la carpeta padre: %v", err)
	}

	return "Carpeta creada con éxito"
}

func ReportFile(id string, outputPath string, filePath string) error {
	// Obtener la ruta del disco usando el id de la partición
	partitionPath := DiskManagement.GetPartitionPathByID(id)
	if partitionPath == "" {
		return fmt.Errorf("no se encontró la ruta para el id: %s", id)
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(partitionPath)
	if err != nil {
		return fmt.Errorf("error abriendo el disco: %v", err)
	}
	defer file.Close()

	// Obtener el inicio de la partición
	partitionStart := DiskManagement.GetPartitionStartByID(id)
	if partitionStart < 0 {
		return fmt.Errorf("no se encontró la partición para el id: %s", id)
	}

	// Leer el Superblock desde el inicio de la partición
	var sb Structs.Superblock
	if err := Utilities.ReadObject(file, &sb, partitionStart); err != nil {
		return fmt.Errorf("error leyendo el Superblock: %v", err)
	}

	// Obtener el inodo del archivo usando su ruta en el sistema ext2
	inode, _ := GetInodeFromPath(filePath, file, sb)
	if inode == nil {
		return fmt.Errorf("no se encontró el archivo: %s", filePath)
	}

	// Obtener el contenido completo del archivo
	content := GetInodeFileData(*inode, file, sb)

	// Crear el reporte: se incluye el nombre del archivo y su contenido.
	reportContent := fmt.Sprintf("Reporte de Archivo\nDirectorio: %s\n\nContenido:\n%s", filePath, content)

	// Guardar el reporte en un archivo de texto en outputPath
	if err := os.WriteFile(outputPath, []byte(reportContent), 0644); err != nil {
		return fmt.Errorf("error escribiendo el reporte: %v", err)
	}

	fmt.Printf("Reporte generado exitosamente en: %s\n", outputPath)
	return nil
}

// parseSinglePerm convierte un dígito (0..7) a 'rwx' parcial.
func parseSinglePerm(value int) string {
	// value es octal, e.g. 7 -> rwx, 5 -> r-x, etc.
	// 4 -> r, 2 -> w, 1 -> x
	var result [3]byte
	result[0] = '-' // r
	result[1] = '-' // w
	result[2] = '-' // x
	if value&4 != 0 {
		result[0] = 'r'
	}
	if value&2 != 0 {
		result[1] = 'w'
	}
	if value&1 != 0 {
		result[2] = 'x'
	}
	return string(result[:])
}

func parsePermissions(perm string, inodoType byte) string {
	// Ejemplo: si perm = "755", convertir a "rwxr-xr-x"
	// Si es directorio, el primer caracter se puede forzar a 'd'
	if len(perm) < 3 {
		// Asumir "000"
		perm = "000"
	}
	// perm[0] -> permisos de propietario
	// perm[1] -> permisos de grupo
	// perm[2] -> permisos de otros
	owner, _ := strconv.Atoi(string(perm[0]))
	group, _ := strconv.Atoi(string(perm[1]))
	other, _ := strconv.Atoi(string(perm[2]))

	ownerStr := parseSinglePerm(owner)
	groupStr := parseSinglePerm(group)
	otherStr := parseSinglePerm(other)

	// Si es un directorio, primer char 'd', si es archivo, '-'
	var prefixChar string
	if inodoType == '0' {
		prefixChar = "d"
	} else {
		prefixChar = "-"
	}
	return prefixChar + ownerStr + groupStr + otherStr
}

// ReportLs genera un reporte Graphviz (DOT y PNG) que muestra la información (permisos, UID, GID, tamaño,
// fechas de creación y modificación, tipo y nombre) de los archivos y carpetas en la ruta 'path_file_ls'
// dentro del sistema ext2 de la partición identificada por 'id'. El reporte se guarda en 'outputPath'.
func ReportLs(id string, outputPath string, path_file_ls string) error {
	// 1. Obtener la ruta del disco a partir del id
	partitionPath := DiskManagement.GetPartitionPathByID(id)
	if partitionPath == "" {
		OutPut.Println("no se encontró la ruta para el id: %s" + id)
		return fmt.Errorf("no se encontró la ruta para el id: %s", id)
	}
	file, err := Utilities.OpenFile(partitionPath)
	if err != nil {
		OutPut.Println("error abriendo el disco: ")
		return fmt.Errorf("error abriendo el disco: %v", err)
	}
	defer file.Close()

	// 2. Obtener el inicio de la partición
	partitionStart := DiskManagement.GetPartitionStartByID(id)
	if partitionStart < 0 {
		return fmt.Errorf("no se encontró la partición con el id: %s", id)
	}

	// 3. Leer el Superblock desde el inicio de la partición
	var sb Structs.Superblock
	if err := Utilities.ReadObject(file, &sb, partitionStart); err != nil {
		return fmt.Errorf("error leyendo el Superblock: %v", err)
	}

	// 4. Obtener el inodo del directorio a listar (path_file_ls)
	inode, _ := GetInodeFromPath(path_file_ls, file, sb)
	if inode == nil {
		OutPut.Println("no se encontró la ruta en el sistema ext2:" + path_file_ls)
		return fmt.Errorf("no se encontró la ruta en el sistema ext2: %s", path_file_ls)
	}

	// Verificar que el inodo corresponda a un directorio (I_type[0] == '0')
	if inode.I_type[0] != '0' {
		OutPut.Println("la ruta especificada no es un directorio: " + path_file_ls)
		return fmt.Errorf("la ruta especificada no es un directorio: %s", path_file_ls)
	}

	// 5. Leer el FolderBlock del directorio (se asume que está en I_block[0])
	if inode.I_block[0] == -1 {
		return fmt.Errorf("el directorio no tiene bloque asignado")
	}
	folder, err := ReadFolderBlock(file, sb, inode.I_block[0])
	if err != nil {
		return fmt.Errorf("error leyendo el FolderBlock: %v", err)
	}

	// 6. Para cada entrada (omitimos "." y ".."), obtener su información (inodo y datos relevantes)
	type EntryInfo struct {
		Name         string
		Perms        string
		UID          int32
		GID          int32
		Size         int32
		CreationDate string
		ModDate      string
		Type         string
	}
	var entries []EntryInfo

	for _, entry := range folder.B_content {
		entryName := strings.Trim(string(entry.B_name[:]), "\x00")
		if entryName == "" || entryName == "." || entryName == ".." {
			continue
		}
		childIndex := int(entry.B_inodo)
		childInode, _ := GetInodeFromPathByIndex(childIndex, file, sb)
		if childInode == nil {
			continue
		}
		// Convertir permisos numéricos (ej. "664") a formato simbólico (ej. "rw-rw-r--")
		perms := parsePermissions(strings.Trim(string(childInode.I_perm[:]), "\x00"), childInode.I_type[0])
		// Usar I_ctime como fecha de creación y I_mtime como fecha de modificación.
		creation := strings.Trim(string(childInode.I_ctime[:]), "\x00")
		modification := strings.Trim(string(childInode.I_mtime[:]), "\x00")
		// Para propósitos de este reporte, podríamos formatear las fechas si es necesario.
		// Aquí las dejamos como strings.
		var tipo string
		if childInode.I_type[0] == '0' {
			tipo = "Directorio"
		} else {
			tipo = "Archivo"
		}
		entries = append(entries, EntryInfo{
			Name:         entryName,
			Perms:        perms,
			UID:          childInode.I_uid,
			GID:          childInode.I_gid,
			Size:         childInode.I_size,
			CreationDate: creation,
			ModDate:      modification,
			Type:         tipo,
		})
	}

	// 7. Generar el reporte DOT usando una tabla HTML (cada fila es un archivo o directorio)
	var buffer strings.Builder
	buffer.WriteString("digraph Reporte_Ls {\n")
	buffer.WriteString("    rankdir=TB;\n")
	buffer.WriteString("    node [fontname=\"Arial\", shape=plaintext, fontsize=10];\n")
	buffer.WriteString("    graph [bgcolor=\"#ffffff\"];\n\n")
	buffer.WriteString("    ls_report [\n")
	buffer.WriteString("        label=<\n")
	buffer.WriteString("            <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\">\n")
	buffer.WriteString("                <TR bgcolor=\"#B3D9FF\">\n")
	buffer.WriteString("                    <TD><B>PERMISOS</B></TD>\n")
	buffer.WriteString("                    <TD><B>UID</B></TD>\n")
	buffer.WriteString("                    <TD><B>GID</B></TD>\n")
	buffer.WriteString("                    <TD><B>TAMAÑO</B></TD>\n")
	buffer.WriteString("                    <TD><B>CREACIÓN</B></TD>\n")
	buffer.WriteString("                    <TD><B>MODIFICACIÓN</B></TD>\n")
	buffer.WriteString("                    <TD><B>TIPO</B></TD>\n")
	buffer.WriteString("                    <TD><B>NOMBRE</B></TD>\n")
	buffer.WriteString("                </TR>\n")

	// Agregar una fila por cada entrada
	for _, e := range entries {
		buffer.WriteString("                <TR>\n")
		buffer.WriteString(fmt.Sprintf("                    <TD>%s</TD>\n", e.Perms))
		buffer.WriteString(fmt.Sprintf("                    <TD>%d</TD>\n", e.UID))
		buffer.WriteString(fmt.Sprintf("                    <TD>%d</TD>\n", e.GID))
		buffer.WriteString(fmt.Sprintf("                    <TD>%d</TD>\n", e.Size))
		buffer.WriteString(fmt.Sprintf("                    <TD>%s</TD>\n", e.CreationDate))
		buffer.WriteString(fmt.Sprintf("                    <TD>%s</TD>\n", e.ModDate))
		buffer.WriteString(fmt.Sprintf("                    <TD>%s</TD>\n", e.Type))
		buffer.WriteString(fmt.Sprintf("                    <TD>%s</TD>\n", e.Name))
		buffer.WriteString("                </TR>\n")
	}

	buffer.WriteString("            </TABLE>\n")
	buffer.WriteString("        >\n")
	buffer.WriteString("    ];\n")
	buffer.WriteString("}\n")

	// 8. Guardar el archivo DOT y generar la imagen
	dotFile := outputPath + ".dot"
	if err := os.WriteFile(dotFile, []byte(buffer.String()), 0644); err != nil {
		return fmt.Errorf("error escribiendo el DOT: %v", err)
	}
	cmd := exec.Command("dot", "-Tpng", "-Gdpi=300", dotFile, "-o", outputPath+".png")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error al ejecutar Graphviz: %v", err)
	}

	fmt.Printf("Reporte LS generado exitosamente en: %s\n", outputPath+".png")
	return nil
}

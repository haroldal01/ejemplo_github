package DiskManagement

import (
	"MIA_P1/OutPut"
	"MIA_P1/Structs"
	"MIA_P1/Utilities"
	"MIA_P1/stores"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var diskCounter int = 0 // contador para generar los nombres de los discos

type MountedPartition struct {
	Path     string
	Name     string
	ID       string
	Status   byte  // 0: no montada, 1: montada
	LoggedIn bool  // true: usuario ha iniciado sesión, false: no ha iniciado sesión
	Start    int64 // Nuevo campo para indicar el offset de inicio de la partición en el disco
}

func Mount(driveLetter string, name string) {
	OutPut.Println("======Start MOUNT======")
	OutPut.Println("Drive Letter:", driveLetter, "Name:", name)

	// Normalize name and drive letter
	name = strings.ToUpper(name)
	driveLetter = strings.ToUpper(driveLetter)

	// Ruta del disco
	filepath := fmt.Sprintf("./tets/%s.dsk", strings.ToUpper(driveLetter))
	file, err := Utilities.OpenFile(filepath)
	if err != nil {
		OutPut.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	var tempMBR Structs.MRB
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		OutPut.Println("Error reading MRB from file:", err)
		return
	}

	// Buscar la partición y contar cuántas ya están montadas
	var index int = -1
	var count int = 1
	var emptyId [4]byte

	for i := 0; i < 4; i++ {
		partName := strings.Trim(string(tempMBR.Partitions[i].Name[:]), "\x00")
		if strings.ToUpper(partName) == name && tempMBR.Partitions[i].Size != 0 {
			if tempMBR.Partitions[i].Id != emptyId {
				OutPut.Println("Error: Partition already mounted")
				return
			}
			index = i
		}
		if tempMBR.Partitions[i].Id != emptyId {
			count++
		}
	}

	if index == -1 {
		OutPut.Println("Error: Partition not found")
		return
	}

	// Crear ID único: Letra + correlativo + carnet
	id := fmt.Sprintf("%s%d%s", strings.ToUpper(driveLetter), count, stores.Carnet)

	// Asignar ID y marcar como montada en el MBR
	copy(tempMBR.Partitions[index].Id[:], id)
	copy(tempMBR.Partitions[index].Status[:], "1")

	// Guardar MBR actualizado
	if err := Utilities.WriteObject(file, tempMBR, 0); err != nil {
		OutPut.Println("Error writing MRB to file:", err)
		return
	}

	// Guardar en el mapa de particiones montadas
	disk := strings.ToUpper(driveLetter)
	newPart := MountedPartition{
		Path:     filepath,
		Name:     name,
		ID:       id,
		Status:   '1',
		LoggedIn: false,
		Start:    int64(tempMBR.Partitions[index].Start),
	}
	mountedPartitions[disk] = append(mountedPartitions[disk], newPart)

	OutPut.Println("Partition mounted successfully")
	Structs.PrintPartition(tempMBR.Partitions[index])
	OutPut.Println("======End MOUNT======")
}

var mountedPartitions = make(map[string][]MountedPartition) // disco → lista de particiones montadas
func GetMountedPartitions() map[string][]MountedPartition {
	return mountedPartitions
}

func Mkdisk(size int, fit string, unit string) {
	OutPut.Println("======Start MKDISK======")
	OutPut.Println("Size:", size, "Fit:", fit, "Unit:", unit)

	// Validate fit
	if fit != "BF" && fit != "FF" && fit != "WF" {
		OutPut.Println("Error: Fit must be BF, FF, or WF")
		return
	}

	// Valida el tamaño
	if size <= 0 {
		OutPut.Println("Error: Size must be greater than 0")
		return
	}

	// Valida las unidades
	if unit != "K" && unit != "M" {
		OutPut.Println("Error: Unit must be K or M")
		return
	}

	// Genera el nombre del disco
	diskCounter++
	diskLetter := string(rune('A' + diskCounter - 1))
	basePath := "./tets"
	filepath := fmt.Sprintf("%s/%s.dsk", basePath, diskLetter)

	// Crea el archivo binario
	err := Utilities.CreateFile(filepath)
	if err != nil {
		OutPut.Println("Error creating file:", err)
		return
	}

	//abre el archivo
	file, err := Utilities.OpenFile(filepath)
	if err != nil {
		OutPut.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	// configura el tamaño del disco
	if unit == "k" || unit == "K" {
		size *= 1024
	} else {
		size *= 1024 * 1024
	}

	// escribe el buffer de ceros
	zeroBuffer := make([]byte, 1024)
	for i := 0; i < size/1024; i++ {
		if err := Utilities.WriteObject(file, zeroBuffer, int64(i*1024)); err != nil {
			OutPut.Println("Error writing to file:", err)
			return
		}
	}

	// Create and write MRB
	var newMRB Structs.MRB
	newMRB.MbrSize = int64(size)
	newMRB.Signature = rand.Int31()
	copy(newMRB.Fit[:], strings.ToUpper(fit))
	copy(newMRB.CreationDate[:], time.Now().Format("2006-01-02 15:04:05"))
	if err := Utilities.WriteObject(file, newMRB, 0); err != nil {
		OutPut.Println("Error writing MRB to file:", err)
		return
	}

	// Read and verify MRB
	var tempMBR Structs.MRB
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		OutPut.Println("Error reading MRB from file:", err)
		return
	}

	OutPut.Println("File size:", tempMBR.MbrSize)
	OutPut.Println("Fit:", string(tempMBR.Fit[:]))
	OutPut.Println("Creation date:", string(tempMBR.CreationDate[:]))
	OutPut.Println("Signature:", tempMBR.Signature)
	OutPut.Println("======End MKDISK======")
}

func GetPartitionPathByID(partitionID string) string {
	for _, partitions := range mountedPartitions {
		for _, partition := range partitions {
			if partition.ID == partitionID {
				return partition.Path
			}
		}
	}
	return ""
}

func GetPartitionStartByID(id string) int64 {
	target := strings.TrimSpace(id)
	mountedPartitions := GetMountedPartitions()
	for _, group := range mountedPartitions {
		for _, partition := range group {
			// Convertir el arreglo de bytes a string y limpiar espacios y caracteres nulos.
			pid := strings.TrimSpace(string(partition.ID[:]))
			if pid == target {
				return int64(partition.Start)
			}
		}
	}
	// Si no se encontró, retorna -1
	return -1
}

func Fdisk(size int, driveLetter string, name string, type_ string, fit string, delete string, unit string, add int) {
	OutPut.Println("======Start FDISK======")
	OutPut.Println("Size:", size, "Drive Letter:", driveLetter, "Name:", name, "Type:", type_, "Fit:", fit, "Unit:", unit, "Add:", add)

	// Validaciones básicas
	if fit != "B" && fit != "F" && fit != "W" {
		OutPut.Println("Error: Fit must be B, F, or W")
		return
	}
	if type_ != "P" && type_ != "E" {
		OutPut.Println("Error: Type must be P or E")
		return
	}
	if delete != "" && delete != "FULL" {
		OutPut.Println("Error: Delete must be FULL")
		return
	}
	//if size <= 0 && delete == "" && add == 0 {
	if size <= 0 && add == 0 {
		OutPut.Println("Error: Size must be greater than 0")
		return
	}
	if unit != "B" && unit != "K" && unit != "M" {
		OutPut.Println("Error: Unit must be B, K, or M")
		return
	}

	// Convertir tamaño y add a bytes
	sizeBytes := int64(size)
	addBytes := int64(add)
	unitFactor := int64(1)
	if unit == "K" {
		unitFactor = 1024
	} else if unit == "M" {
		unitFactor = 1024 * 1024
	}
	sizeBytes *= unitFactor
	addBytes *= unitFactor

	// Abrir archivo
	filepath := fmt.Sprintf("./tets/%s.dsk", strings.ToUpper(driveLetter))
	file, err := Utilities.OpenFile(filepath)
	if err != nil {
		OutPut.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	// Read MBR
	var tempMBR Structs.MRB
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		OutPut.Println("Error reading MRB from file:", err)
		return
	}

	// Manejar -add
	if add != 0 {
		for i := 0; i < 4; i++ {
			if strings.Trim(string(tempMBR.Partitions[i].Name[:]), "\x00") == name {
				if addBytes < 0 {
					// Validar que no quede tamaño negativo
					if tempMBR.Partitions[i].Size+addBytes <= 0 {
						OutPut.Println("Error: No se puede reducir tanto el tamaño, resultaría en tamaño negativo")
						return
					}
					tempMBR.Partitions[i].Size += addBytes
				} else {
					// Validar que haya espacio libre después de la partición
					end := tempMBR.Partitions[i].Start + tempMBR.Partitions[i].Size
					available := true
					for j := 0; j < 4; j++ {
						if i != j && tempMBR.Partitions[j].Size != 0 && tempMBR.Partitions[j].Start > tempMBR.Partitions[i].Start {
							if tempMBR.Partitions[j].Start < end+addBytes {
								available = false
								break
							}
						}
					}
					if !available {
						OutPut.Println("Error: No hay espacio contiguo suficiente para expandir la partición")
						return
					}
					tempMBR.Partitions[i].Size += addBytes
				}

				if err := Utilities.WriteObject(file, tempMBR, 0); err != nil {
					OutPut.Println("Error writing MRB:", err)
					return
				}
				OutPut.Println("Partición actualizada correctamente")
				Structs.PrintMBR(tempMBR)
				return
			}
		}
		OutPut.Println("Error: Partición no encontrada para aplicar -add")
		return
	}

	//=======================================================================
	// Check for duplicate name
	for i := 0; i < 4; i++ {
		if strings.Trim(string(tempMBR.Partitions[i].Name[:]), "\x00") == name && tempMBR.Partitions[i].Size != 0 && delete == "" {
			OutPut.Println("Error: Partition name already exists")
			return
		}
	}

	// Handle deletion
	if delete != "" {
		for i := 0; i < 4; i++ {
			if strings.Trim(string(tempMBR.Partitions[i].Name[:]), "\x00") == name && tempMBR.Partitions[i].Size != 0 {
				fmt.Printf("Confirm deletion of partition %s? (y/n): ", name)
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" {
					OutPut.Println("Deletion cancelled")
					return
				}
				tempMBR.Partitions[i] = Structs.Partition{}
				if err := Utilities.WriteObject(file, tempMBR, 0); err != nil {
					OutPut.Println("Error writing MRB:", err)
					return
				}
				OutPut.Println("Partition deleted successfully")
				Structs.PrintMBR(tempMBR)
				return
			}
		}
		OutPut.Println("Error: Partition not found")
		return
	}

	// Check extended partition limit
	extendedCount := 0
	for i := 0; i < 4; i++ {
		if string(tempMBR.Partitions[i].Type[:]) == "e" && tempMBR.Partitions[i].Size != 0 {
			extendedCount++
		}
	}
	if type_ == "e" && extendedCount > 0 {
		OutPut.Println("Error: Only one extended partition allowed")
		return
	}

	// Find free space and create partition
	gap := int64(binary.Size(Structs.MRB{}))
	for i := 0; i < 4; i++ {
		if tempMBR.Partitions[i].Size != 0 && tempMBR.Partitions[i].Start >= gap {
			gap = tempMBR.Partitions[i].Start + tempMBR.Partitions[i].Size
		}
	}

	var index int = -1
	for i := 0; i < 4; i++ {
		if tempMBR.Partitions[i].Size == 0 {
			index = i
			tempMBR.Partitions[i].Size = sizeBytes
			copy(tempMBR.Partitions[i].Name[:], name)
			copy(tempMBR.Partitions[i].Fit[:], fit)
			copy(tempMBR.Partitions[i].Status[:], "0")
			copy(tempMBR.Partitions[i].Type[:], type_)
			tempMBR.Partitions[i].Start = gap
			break
		}
	}

	if index == -1 {
		OutPut.Println("Error: No empty partition found")
		return
	}

	// Write updated MBR
	if err := Utilities.WriteObject(file, tempMBR, 0); err != nil {
		OutPut.Println("Error writing MRB to file:", err)
		return
	}
	//=======================================================================

	Structs.PrintMBR(tempMBR)
	OutPut.Println("======End FDISK======")
}

func Rmdisk(driveLetter string, confirm bool) string {
	OutPut.Println("======Start RMDISK======")
	OutPut.Println("Drive Letter:", driveLetter)

	filepath := fmt.Sprintf("./tets/%s.dsk", strings.ToUpper(driveLetter))
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		return "Error: Disk does not exist"
	}

	if !confirm {
		return "CONFIRM_RMDISK: ¿Está seguro que desea eliminar el disco " + driveLetter + ".dsk?"
	}

	if err := os.Remove(filepath); err != nil {
		return fmt.Sprintf("Error deleting disk: %v", err)
	}

	OutPut.Println("Disk deleted successfully")
	OutPut.Println("======End RMDISK======")
	return "Disk deleted successfully"
}

func Unmount(id string) {
	OutPut.Println("======Start UNMOUNT======")
	OutPut.Println("ID:", id)

	_, diskPath, err := stores.GetMountedPartition(id)
	if err != nil {
		OutPut.Println("Error:", err)
		return
	}

	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		OutPut.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	var tempMBR Structs.MRB
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		OutPut.Println("Error reading MRB:", err)
		return
	}

	for i := 0; i < 4; i++ {
		if strings.Trim(string(tempMBR.Partitions[i].Id[:]), "\x00") == id {
			tempMBR.Partitions[i].Status = [1]byte{'0'}
			tempMBR.Partitions[i].Id = [4]byte{}
			if err := Utilities.WriteObject(file, tempMBR, 0); err != nil {
				OutPut.Println("Error writing MRB:", err)
				return
			}
			OutPut.Println("Partition unmounted successfully")
			Structs.PrintMBR(tempMBR)
			OutPut.Println("======End UNMOUNT======")
			return
		}
	}

	OutPut.Println("Error: Partition not found")
	OutPut.Println("======End UNMOUNT======")
}

// ReportMBR genera un reporte del MBR y lo guarda en la ruta especificada
func ReportMBR(mbr *Structs.MRB, path string) error {
	// Crear las carpetas padre si no existen
	err := Utilities.CreateParentDirs(path)
	if err != nil {
		return err
	}

	// Obtener el nombre base del archivo sin la extensión
	dotFileName, OutPutImage := Utilities.GetFileNames(path)

	// Definir el contenido DOT con una tabla
	dotContent := fmt.Sprintf(`digraph G {
        node [shape=plaintext]
        tabla [label=<
            <table border="0" cellborder="1" cellspacing="0">
                <tr><td colspan="2"> REPORTE MBR </td></tr>
                <tr><td>mbr_tamano</td><td>%d</td></tr>
                <tr><td>mrb_fecha_creacion</td><td>%s</td></tr>
                <tr><td>mbr_disk_signature</td><td>%d</td></tr>
			`, mbr.MbrSize, strings.TrimRight(string(mbr.CreationDate[:]), "\x00"), mbr.Signature)

	// Agregar las particiones a la tabla
	for i, part := range mbr.Partitions {
		/*
			// Continuar si el tamaño de la partición es -1 (o sea, no está asignada)
			if part.Part_size == -1 {
				continue
			}
		*/

		// Convertir Part_name a string y eliminar los caracteres nulos
		partName := strings.TrimRight(string(part.Name[:]), "\x00")
		// Convertir Part_status, Part_type y Part_fit a char
		partStatus := rune(part.Status[0])
		partType := rune(part.Type[0])
		partFit := rune(part.Fit[0])

		// Agregar la partición a la tabla
		dotContent += fmt.Sprintf(`
				<tr><td colspan="2"> PARTICIÓN %d </td></tr>
				<tr><td>part_status</td><td>%c</td></tr>
				<tr><td>part_type</td><td>%c</td></tr>
				<tr><td>part_fit</td><td>%c</td></tr>
				<tr><td>part_start</td><td>%d</td></tr>
				<tr><td>part_size</td><td>%d</td></tr>
				<tr><td>part_name</td><td>%s</td></tr>
			`, i+1, partStatus, partType, partFit, part.Start, part.Size, partName)
	}

	// Cerrar la tabla y el contenido DOT
	dotContent += "</table>>] }"

	// Guardar el contenido DOT en un archivo
	file, err := os.Create(dotFileName)
	if err != nil {
		return fmt.Errorf("error al crear el archivo: %v", err)
	}
	defer file.Close()

	_, err = file.WriteString(dotContent)
	if err != nil {
		return fmt.Errorf("error al escribir en el archivo: %v", err)
	}

	// Ejecutar el comando Graphviz para generar la imagen
	cmd := exec.Command("dot", "-Tpng", dotFileName, "-o", OutPutImage)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error al ejecutar el comando Graphviz: %v", err)
	}

	OutPut.Println("Imagen de la tabla generada:", OutPutImage)
	return nil
}

// DiskReport
func DiskReport(id string, OutPutPath string) error {
	// 1. Obtain disk path from partition ID.
	diskPath := GetPartitionPathByID(id)
	if diskPath == "" {
		return errors.New("no se encontró una partición montada con ID " + id)
	}

	// 2. Open the disk file.
	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return fmt.Errorf("no se pudo abrir el archivo: %v", err)
	}
	defer file.Close()

	// 3. Read the MBR.
	var mbr Structs.MRB
	if err := Utilities.ReadObject(file, &mbr, 0); err != nil {
		return fmt.Errorf("no se pudo leer el MBR: %v", err)
	}

	// 4. Define struct for partition information (primary only).
	type partitionInfo struct {
		Name   string
		Size   int32
		Start  int32
		Type   byte
		Fit    byte
		Status byte
	}

	// 5. Collect active primary partitions (Status='1' or '0').
	var partitions []partitionInfo
	for i := 0; i < 4; i++ {
		p := mbr.Partitions[i]
		if (p.Status[0] == '1' || p.Status[0] == '0') && (p.Type[0] == 'p' || p.Type[0] == 'P') {
			pi := partitionInfo{
				Name:   strings.TrimRight(string(p.Name[:]), "\x00 "),
				Size:   int32(p.Size),
				Start:  int32(p.Start),
				Type:   p.Type[0],
				Fit:    p.Fit[0],
				Status: p.Status[0],
			}
			partitions = append(partitions, pi)
		}
	}

	// 6. Sort partitions by start position.
	sort.Slice(partitions, func(i, j int) bool {
		return partitions[i].Start < partitions[j].Start
	})

	// 7. Calculate used and free space.
	usedSpace := int32(0)
	for _, pi := range partitions {
		usedSpace += pi.Size
	}
	freeSpace := int32(mbr.MbrSize) - usedSpace

	// 8. Initialize DOT content.
	var buffer strings.Builder
	buffer.WriteString(`digraph DISK_Report {
    rankdir=LR;
    graph [bgcolor="#ffffff", fontsize=12, labelloc="t", labeljust="c"];
    node [shape=none];
    `)

	// Add disk title with name and total size.
	fileName := diskPath
	if lastSlash := strings.LastIndex(diskPath, "/"); lastSlash != -1 {
		fileName = diskPath[lastSlash+1:]
	}
	buffer.WriteString(fmt.Sprintf(`label="DISCO: %s (Tamaño Total: %d bytes)";`, fileName, mbr.MbrSize))
	buffer.WriteString("\n")

	// Function to calculate percentage.
	toPercent := func(size, total int32) string {
		if total == 0 {
			return "0%"
		}
		return fmt.Sprintf("%.2f%%", float64(size)*100.0/float64(total))
	}

	// Calculate MBR size.
	mbrSize := int32(binary.Size(mbr))

	// 9. Create node with table for partitions.
	buffer.WriteString("disk [label=<\n")
	buffer.WriteString(`<TABLE BORDER="1" CELLBORDER="1" CELLSPACING="0" CELLPADDING="4">
    <TR>`)

	// Add MBR cell.
	buffer.WriteString(fmt.Sprintf(`
        <TD BGCOLOR="#B3E5FC" PORT="mbr">
            <B>MBR</B><br/>
            Inicio: 0<br/>
            Tamaño: %d bytes<br/>
        </TD>
    `, mbrSize))

	// 10. Add cells for primary partitions.
	for _, pi := range partitions {
		percentage := toPercent(pi.Size, int32(mbr.MbrSize))
		bgColor := "#E8F5E9" // Green for primary partitions.
		title := fmt.Sprintf("Primaria<br/>%s<br/>(%s)", pi.Name, percentage)
		buffer.WriteString(fmt.Sprintf(`
        <TD BGCOLOR="%s" PORT="%s">
            <B>%s</B><br/>
            Inicio: %d<br/>
            Tamaño: %d bytes
        </TD>
        `, bgColor, pi.Name, title, pi.Start, pi.Size))
	}

	// 11. Add free space cell if applicable.
	if freeSpace > 0 {
		freePercent := toPercent(freeSpace, int32(mbr.MbrSize))
		buffer.WriteString(fmt.Sprintf(`
        <TD BGCOLOR="#ECEFF1">
            <B>Libre</B><br/>
            %d bytes<br/>
            (%s)
        </TD>`, freeSpace, freePercent))
	}

	buffer.WriteString("</TR>\n</TABLE>\n>];\n")

	// 12. Close the graph.
	buffer.WriteString("}\n")

	// 13. Write DOT file to disk.
	dotFile := OutPutPath + ".dot"
	if err := os.WriteFile(dotFile, []byte(buffer.String()), 0644); err != nil {
		return fmt.Errorf("no se pudo escribir el archivo DOT: %v", err)
	}

	// 14. Execute Graphviz to generate PNG.
	cmd := exec.Command("dot", "-Tpng", "-Gdpi=300", dotFile, "-o", OutPutPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error al ejecutar dot: %v", err)
	}
	OutPut.Println("Reporte de disco generado en:", OutPutPath)
	return nil
}

func InodeReport(id string, OutPutPath string) error {
	// Obtener la ruta del disco
	partitionPath := GetPartitionPathByID(id)
	if partitionPath == "" {
		return fmt.Errorf("no se encontró la ruta para el id: %s", id)
	}
	file, err := Utilities.OpenFile(partitionPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Obtener el inicio de la partición
	partitionStart := GetPartitionStartByID(id)
	fmt.Printf("Partition start: %d\n", partitionStart)

	if partitionStart < 0 {
		return fmt.Errorf("no se encontró la partición con el id %s", id)
	}

	// Leer el superblock desde el inicio de la partición
	var sb Structs.Superblock
	if err := Utilities.ReadObject(file, &sb, partitionStart); err != nil {
		return err
	}
	fmt.Printf("Superblock leído: S_inode_start=%d\n", sb.S_inode_start)

	// Aquí usamos S_inode_start directamente (ya es absoluto)
	inodeStart := int64(sb.S_inode_start)
	inodeSize := int64(sb.S_inode_size)
	count := sb.S_inodes_count

	// Obtener tamaño del archivo para verificar límites
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	fileSize := fileInfo.Size()

	// Generar reporte DOT
	var buffer strings.Builder
	buffer.WriteString("digraph Inode_Report {\n")
	buffer.WriteString("    rankdir=TB;\n")
	buffer.WriteString("    node [fontname=\"Arial\", shape=plain, style=\"filled\", fontsize=10];\n")
	buffer.WriteString("    edge [dir=none];\n")
	buffer.WriteString("    graph [bgcolor=\"#ffffff\", pencolor=\"#333333\", penwidth=2.0, style=\"rounded\"];\n\n")

	var lastInodeIndex int = -1 // Para almacenar el índice del último inodo válido

	// Recorrer la tabla de inodos.
	for i := 0; i < int(count); i++ {
		pos := inodeStart + (inodeSize * int64(i))
		if pos+inodeSize > fileSize {
			fmt.Printf("Se alcanzó el final del archivo en el inodo %d (offset: %d, archivo: %d bytes)\n", i, pos, fileSize)
			break
		}
		var inode Structs.Inode
		if err := Utilities.ReadObject(file, &inode, pos); err != nil {
			return err
		}
		// Verificar si el inodo está en uso
		used := inode.I_size > 0
		for _, b := range inode.I_block {
			if b != -1 {
				used = true
				break
			}
		}
		if !used {
			continue
		}

		// Construir cadena de bloques asignados
		blocksStr := ""
		for j, b := range inode.I_block {
			if b != -1 {
				blocksStr += fmt.Sprintf("Bloque%d: %d\\n", j, b)
			}
		}
		atime := strings.Trim(string(inode.I_atime[:]), "\x00")
		ctime := strings.Trim(string(inode.I_ctime[:]), "\x00")
		mtime := strings.Trim(string(inode.I_mtime[:]), "\x00")
		perm := strings.Trim(string(inode.I_perm[:]), "\x00")

		// Construcción del nodo inodo en DOT
		buffer.WriteString(fmt.Sprintf("    inode%d [\n", i))
		buffer.WriteString("        fillcolor=\"#E6F3FF\",\n")
		buffer.WriteString("        label=<\n")
		buffer.WriteString("            <TABLE BORDER=\"0\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\" BGCOLOR=\"#E6F3FF\">\n")
		buffer.WriteString(fmt.Sprintf("                <TR><TD COLSPAN=\"2\" BGCOLOR=\"#B3D9FF\"><B>Inodo %d</B></TD></TR>\n", i))
		buffer.WriteString(fmt.Sprintf("                <TR><TD><B>UID</B></TD><TD>%d</TD></TR>\n", inode.I_uid))
		buffer.WriteString(fmt.Sprintf("                <TR><TD><B>GID</B></TD><TD>%d</TD></TR>\n", inode.I_gid))
		buffer.WriteString(fmt.Sprintf("                <TR><TD><B>Tamaño</B></TD><TD>%d bytes</TD></TR>\n", inode.I_size))
		buffer.WriteString(fmt.Sprintf("                <TR><TD><B>ATime</B></TD><TD>%s</TD></TR>\n", atime))
		buffer.WriteString(fmt.Sprintf("                <TR><TD><B>CTime</B></TD><TD>%s</TD></TR>\n", ctime))
		buffer.WriteString(fmt.Sprintf("                <TR><TD><B>MTime</B></TD><TD>%s</TD></TR>\n", mtime))
		buffer.WriteString(fmt.Sprintf("                <TR><TD><B>Permisos</B></TD><TD>%s</TD></TR>\n", perm))
		buffer.WriteString(fmt.Sprintf("                <TR><TD><B>Bloques</B></TD><TD>%s</TD></TR>\n", blocksStr))
		buffer.WriteString("            </TABLE>\n")
		buffer.WriteString("        >\n")
		buffer.WriteString("    ];\n\n")

		// Si hay un inodo previo válido, conectar con el actual
		if lastInodeIndex != -1 {
			buffer.WriteString(fmt.Sprintf("    inode%d -> inode%d;\n", lastInodeIndex, i))
		}
		lastInodeIndex = i // Guardar el índice del último inodo válido
	}
	buffer.WriteString("}\n")

	// Guardar el archivo DOT
	dotFile := OutPutPath + ".dot"
	if err := os.WriteFile(dotFile, []byte(buffer.String()), 0644); err != nil {
		return err
	}

	// Generar el PNG con Graphviz
	cmd := exec.Command("dot", "-Tpng", "-Gdpi=300", dotFile, "-o", OutPutPath+".png")
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func parseBlock(rawBlock []byte) (string, string) {
	var fblock Structs.Folderblock
	if err := binary.Read(bytes.NewReader(rawBlock), binary.LittleEndian, &fblock); err == nil {
		var label strings.Builder
		for i, content := range fblock.B_content {
			name := strings.TrimRight(string(content.B_name[:]), "\x00")
			if name != "" || content.B_inodo != 0 {
				label.WriteString(fmt.Sprintf("Carpeta[%d]: %s (Inodo: %d)<br/>", i, name, content.B_inodo))
			}
		}
		if label.Len() > 0 {
			return "Directorio", label.String()
		}
	}

	var ffile Structs.Fileblock
	if err := binary.Read(bytes.NewReader(rawBlock), binary.LittleEndian, &ffile); err == nil {
		contentStr := strings.TrimRight(string(ffile.B_content[:]), "\x00")
		if len(contentStr) > 0 && strings.IndexFunc(contentStr, func(r rune) bool { return r < 32 && r != 10 }) == -1 {
			return "Archivo", fmt.Sprintf("Datos (64 bytes): <br/>%s", contentStr)
		}
	}

	var pblock Structs.Pointerblock
	if err := binary.Read(bytes.NewReader(rawBlock), binary.LittleEndian, &pblock); err == nil {
		var label strings.Builder
		for i, ptr := range pblock.B_pointers {
			if ptr > 0 {
				label.WriteString(fmt.Sprintf("Puntero[%d] = %d<br/>", i, ptr))
			}
		}
		if label.Len() > 0 {
			return "PointerBlock", label.String()
		}
	}

	return "Desconocido", "Datos binarios (64 bytes)"
}

func BlockReport(id string, OutPutPath string) error {
	// Obtener la ruta del disco
	partitionPath := GetPartitionPathByID(id)
	if partitionPath == "" {
		return fmt.Errorf("no se encontró la ruta para el id: %s", id)
	}
	file, err := Utilities.OpenFile(partitionPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Obtener el inicio de la partición
	partitionStart := GetPartitionStartByID(id)
	if partitionStart < 0 {
		return fmt.Errorf("no se encontró la partición con el id %s", id)
	}

	// Leer el Superblock desde el inicio de la partición
	var sb Structs.Superblock
	if err := Utilities.ReadObject(file, &sb, partitionStart); err != nil {
		return err
	}
	fmt.Printf("Superblock leído: S_block_start=%d, S_block_size=%d, S_blocks_count=%d\n", sb.S_block_start, sb.S_block_size, sb.S_blocks_count)

	// Tamaños y offsets para lectura
	blockCount := sb.S_blocks_count
	blockSize := int64(sb.S_block_size) // Asegurarse que S_block_size es el tamaño en bytes
	bmBlockStart := int64(sb.S_bm_block_start)
	blockStart := int64(sb.S_block_start)

	// Verificar límites del archivo
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	fileSize := fileInfo.Size()

	// Iniciar construcción del reporte DOT
	var buffer strings.Builder
	buffer.WriteString("digraph Block_Report {\n")
	buffer.WriteString("    rankdir=TB;\n")
	buffer.WriteString("    node [fontname=\"Arial\", shape=plain, style=\"filled\", fontsize=10];\n")
	buffer.WriteString("    edge [color=\"#5A5A5A\", arrowhead=normal];\n")
	buffer.WriteString("    graph [bgcolor=\"#ffffff\", pencolor=\"#333333\", penwidth=2.0, style=\"rounded\"];\n\n")

	lastUsedBlock := -1

	// Recorrer todos los bloques y verificar si están usados en el bitmap
	for i := int32(0); i < blockCount; i++ {
		bmOffset := bmBlockStart + int64(i)
		if bmOffset >= fileSize {
			fmt.Printf("Se alcanzó el final del archivo al leer el bitmap de bloques (índice: %d)\n", i)
			break
		}
		var bitValue byte
		if err := Utilities.ReadObject(file, &bitValue, bmOffset); err != nil {
			return err
		}

		if bitValue == 1 {
			blockPos := blockStart + int64(i)*int64(blockSize)

			// Verificar si el offset está dentro de los límites del archivo
			if blockPos+int64(blockSize) > fileSize {
				fmt.Printf("Se alcanzó el final del archivo en el bloque %d (offset: %d, tamaño archivo: %d)\n", i, blockPos, fileSize)
				break
			}

			fmt.Printf("Leyendo bloque %d en offset %d\n", i, blockPos)
		}

		if bitValue == 1 {
			// Calcular la posición del bloque en el disco
			blockPos := blockStart + int64(i)*blockSize
			if blockPos+blockSize > fileSize {
				fmt.Printf("Se alcanzó el final del archivo en el bloque %d (offset: %d, tamaño archivo: %d bytes)\n", i, blockPos, fileSize)
				break
			}

			// Leer el bloque
			rawBlock := make([]byte, blockSize)
			n, err := file.ReadAt(rawBlock, blockPos)
			if err != nil {
				return fmt.Errorf("error al leer bloque %d: %v", i, err)
			}
			fmt.Printf("📦 Bloque %d contenido (primeros 16 bytes): %x\n", i, rawBlock[:16])
			if int64(n) != blockSize {
				fmt.Printf("Advertencia: leído %d bytes en bloque %d, se esperaba %d bytes\n", n, i, blockSize)
			}

			// Verificar si el bloque es todo ceros
			isEmpty := true
			for _, b := range rawBlock {
				if b != 0 {
					isEmpty = false
					break
				}
			}

			var blockType, blockLabel string
			if isEmpty {
				blockType = "Vacío"
				blockLabel = ""
			} else {
				// Intentar parsear el bloque (usa tu función parseBlock, que debe estar adaptada)
				blockType, blockLabel = parseBlock(rawBlock)
				fmt.Printf("Bloque %d detectado como tipo: %s - %s\n", i, blockType, blockLabel)
			}

			// Log de depuración
			fmt.Printf("Bloque %d: offset %d, Tipo: %s\n", i, blockPos, blockType)

			// Crear nodo DOT para el bloque
			buffer.WriteString(fmt.Sprintf("    block%d [\n", i))
			buffer.WriteString("        label=<\n")
			buffer.WriteString("            <TABLE BORDER=\"0\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\" BGCOLOR=\"#CDEFFA\">\n")
			buffer.WriteString(fmt.Sprintf("                <TR><TD COLSPAN=\"1\" BGCOLOR=\"#92E6F1\"><B>Bloque %d</B></TD></TR>\n", i))
			buffer.WriteString(fmt.Sprintf("                <TR><TD ALIGN=\"LEFT\">Tipo: %s<br/>%s</TD></TR>\n", blockType, blockLabel))
			buffer.WriteString("            </TABLE>\n")
			buffer.WriteString("        >\n")
			buffer.WriteString("    ];\n\n")

			// Conectar con el bloque anterior si existe.
			if lastUsedBlock != -1 {
				buffer.WriteString(fmt.Sprintf("    block%d -> block%d;\n", lastUsedBlock, i))
			}
			lastUsedBlock = int(i)
		}
	}

	buffer.WriteString("}\n")

	// Guardar el archivo DOT
	dotFile := OutPutPath + ".dot"
	if err := os.WriteFile(dotFile, []byte(buffer.String()), 0644); err != nil {
		return err
	}

	// Generar la imagen PNG con Graphviz.
	cmd := exec.Command("dot", "-Tpng", "-Gdpi=300", dotFile, "-o", OutPutPath+".png")
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func GetDiskNameByID(id string) string {
	target := strings.TrimSpace(id)
	mountedPartitions := GetMountedPartitions()

	for diskName, partitions := range mountedPartitions {
		for _, partition := range partitions {
			// Convertir el ID de la partición a string y limpiar espacios.
			pid := strings.TrimSpace(string(partition.ID[:]))
			if pid == target {
				return diskName // Retorna el nombre del disco
			}
		}
	}
	// Si no se encuentra, retorna una cadena vacía
	return ""
}

func BmInodeReport(id string, OutPutPath string) error {
	// 1. Obtener la ruta del disco a partir del id
	partitionPath := GetPartitionPathByID(id)
	if partitionPath == "" {
		return fmt.Errorf("no se encontró la ruta para el id: %s", id)
	}

	// 2. Abrir el archivo
	file, err := Utilities.OpenFile(partitionPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 3. Obtener el inicio de la partición
	partitionStart := GetPartitionStartByID(id)
	if partitionStart < 0 {
		return fmt.Errorf("no se encontró la partición con el id %s", id)
	}

	// 4. Leer el superblock desde el inicio de la partición
	var sb Structs.Superblock
	if err := Utilities.ReadObject(file, &sb, partitionStart); err != nil {
		return err
	}

	// 5. Obtener la cantidad de inodos y el tamaño de cada inodo.
	inodesCount := sb.S_inodes_count
	inodeSize := binary.Size(Structs.Inode{})

	// 6. Generar un bitmap "virtual" escaneando la tabla de inodos.
	// Si el inodo está en uso (I_type[0] != 0) se marca con 1, de lo contrario 0.
	bitmap := make([]byte, inodesCount)
	for i := 0; i < int(inodesCount); i++ {
		offset := int64(sb.S_inode_start) + int64(i)*int64(inodeSize)
		var inode Structs.Inode
		if err := Utilities.ReadObject(file, &inode, offset); err != nil {
			return fmt.Errorf("error al leer el inodo %d: %v", i, err)
		}
		if inode.I_type[0] != 0 {
			bitmap[i] = 1
		} else {
			bitmap[i] = 0
		}
	}

	// 7. Construir la cadena del reporte con 20 registros (bits) por línea.
	var OutPutBuilder strings.Builder
	OutPutBuilder.WriteString(fmt.Sprintf("============= REPORTE BITMAP DE INODOS - PARTICIÓN: %s =============\n", id))
	for i, b := range bitmap {
		OutPutBuilder.WriteString(fmt.Sprintf("%d", b))
		if (i+1)%20 == 0 {
			OutPutBuilder.WriteString("\n")
		}
	}
	if inodesCount%20 != 0 {
		OutPutBuilder.WriteString("\n")
	}

	// 8. Guardar el reporte en un archivo de texto.
	reportFile := OutPutPath + ".txt"
	if err := os.WriteFile(reportFile, []byte(OutPutBuilder.String()), 0644); err != nil {
		return err
	}

	fmt.Printf("Reporte bm_inode generado en: %s\n", reportFile)
	return nil
}

func BmBlockReport(id string, OutPutPath string) error {
	// 1. Obtener la ruta del disco a partir del id
	partitionPath := GetPartitionPathByID(id)
	if partitionPath == "" {
		return fmt.Errorf("no se encontró la ruta para el id: %s", id)
	}

	// 2. Abrir el archivo
	file, err := Utilities.OpenFile(partitionPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 3. Obtener el inicio de la partición
	partitionStart := GetPartitionStartByID(id)
	if partitionStart < 0 {
		return fmt.Errorf("no se encontró la partición con el id %s", id)
	}

	// 4. Leer el superblock desde la partición
	var sb Structs.Superblock
	if err := Utilities.ReadObject(file, &sb, partitionStart); err != nil {
		return err
	}

	// 5. Obtener la cantidad de bloques y el inicio del bitmap de bloques
	blocksCount := sb.S_blocks_count
	bmBlockStart := sb.S_bm_block_start

	// 6. Leer el bitmap completo (cada bloque usa 1 byte en el bitmap)
	bmData := make([]byte, blocksCount)
	_, err = file.ReadAt(bmData, int64(bmBlockStart))
	if err != nil {
		return fmt.Errorf("error al leer bitmap de bloques: %v", err)
	}

	// 7. Construir la cadena con 20 registros por línea
	var OutPutBuilder strings.Builder

	disk := GetDiskNameByID(id)

	OutPutBuilder.WriteString(fmt.Sprintf("=============  REPORTE BITMAP DE BLOQUES  - PARTICION: %s ===================== \n", disk))
	for i, b := range bmData {
		// Cada byte b es 0 ó 1
		OutPutBuilder.WriteString(fmt.Sprintf("%d", b))

		// Cada 20 bits hacemos un salto de línea
		if (i+1)%20 == 0 {
			OutPutBuilder.WriteString("\n")
		}
	}

	// Si la cantidad de bloques no es múltiplo de 20, forzamos un salto de línea final
	if blocksCount%20 != 0 {
		OutPutBuilder.WriteString("\n")
	}

	// 8. Guardar la salida en un archivo de texto
	reportFile := OutPutPath + ".txt"
	if err := os.WriteFile(reportFile, []byte(OutPutBuilder.String()), 0644); err != nil {
		return err
	}

	fmt.Printf("Reporte bm_block generado en: %s\n", reportFile)
	return nil
}

func SuperBlockReport(id string, OutPutPath string) error {
	// 1. Obtener la ruta del disco a partir del id de la partición
	partitionPath := GetPartitionPathByID(id)
	if partitionPath == "" {
		return fmt.Errorf("no se encontró la ruta para el id: %s", id)
	}
	file, err := Utilities.OpenFile(partitionPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 2. Obtener el inicio de la partición
	partitionStart := GetPartitionStartByID(id)
	if partitionStart < 0 {
		return fmt.Errorf("no se encontró la partición con el id %s", id)
	}

	// 3. Leer el superbloque desde el inicio de la partición
	var sb Structs.Superblock
	if err := Utilities.ReadObject(file, &sb, partitionStart); err != nil {
		return err
	}

	// Determinar el tipo de sistema de archivos
	fsType := "EXT2"
	if sb.S_filesystem_type == 3 {
		fsType = "EXT3"
	}
	disk := GetDiskNameByID(id)
	// Calcular inodos y bloques usados.
	usedInodes := sb.S_inodes_count - sb.S_free_inodes_count
	usedBlocks := sb.S_blocks_count - sb.S_free_blocks_count

	// 4. Generar el reporte DOT
	var buffer strings.Builder
	buffer.WriteString("digraph Superblock_Report {\n")
	buffer.WriteString("    rankdir=TB;\n")
	buffer.WriteString("    node [fontname=\"Arial\", shape=plaintext, style=\"filled\", fontsize=10];\n")
	buffer.WriteString("    graph [bgcolor=\"#ffffff\", pencolor=\"#333333\", penwidth=2.0, style=\"rounded\"];\n\n")

	buffer.WriteString("    superblock [\n")
	buffer.WriteString("        label=<\n")
	buffer.WriteString("            <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"6\">\n")
	buffer.WriteString("                <TR><TD COLSPAN=\"2\" BGCOLOR=\"#B3D9FF\"><B>REPORTE DE SUPERBLOQUE</B></TD></TR>\n")
	buffer.WriteString("                <TR><TD COLSPAN=\"2\" BGCOLOR=\"#E6F3FF\"><B>Información General</B></TD></TR>\n")
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Ruta del Disco</TD><TD>%s</TD></TR>\n", disk))
	buffer.WriteString(fmt.Sprintf("                <TR><TD>ID de Partición</TD><TD>%s</TD></TR>\n", id))
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Tipo de Sistema</TD><TD>%s</TD></TR>\n", fsType))
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Valor Mágico</TD><TD>0x%X</TD></TR>\n", sb.S_magic))
	buffer.WriteString("                <TR><TD COLSPAN=\"2\" BGCOLOR=\"#E6F3FF\"><B>Estado del Sistema</B></TD></TR>\n")
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Veces Montado</TD><TD>%d</TD></TR>\n", sb.S_mnt_count))
	buffer.WriteString("                <TR><TD COLSPAN=\"2\" BGCOLOR=\"#E6F3FF\"><B>Uso de Inodos y Bloques</B></TD></TR>\n")
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Total de Inodos</TD><TD>%d</TD></TR>\n", sb.S_inodes_count))
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Inodos Libres</TD><TD>%d</TD></TR>\n", sb.S_free_inodes_count))
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Inodos Usados</TD><TD>%d</TD></TR>\n", usedInodes))
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Total de Bloques</TD><TD>%d</TD></TR>\n", sb.S_blocks_count))
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Bloques Libres</TD><TD>%d</TD></TR>\n", sb.S_free_blocks_count))
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Bloques Usados</TD><TD>%d</TD></TR>\n", usedBlocks))
	buffer.WriteString("                <TR><TD COLSPAN=\"2\" BGCOLOR=\"#E6F3FF\"><B>Detalles de la Estructura</B></TD></TR>\n")
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Tamaño de Inodo</TD><TD>%d bytes</TD></TR>\n", sb.S_inode_size))
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Tamaño de Bloque</TD><TD>%d bytes</TD></TR>\n", sb.S_block_size))
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Primer Inodo Libre</TD><TD>%d</TD></TR>\n", sb.S_fist_ino))
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Primer Bloque Libre</TD><TD>%d</TD></TR>\n", sb.S_first_blo))
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Inicio Bitmap Inodos</TD><TD>%d</TD></TR>\n", sb.S_bm_inode_start))
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Inicio Bitmap Bloques</TD><TD>%d</TD></TR>\n", sb.S_bm_block_start))
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Inicio Tabla Inodos</TD><TD>%d</TD></TR>\n", sb.S_inode_start))
	buffer.WriteString(fmt.Sprintf("                <TR><TD>Inicio Tabla Bloques</TD><TD>%d</TD></TR>\n", sb.S_block_start))
	buffer.WriteString("            </TABLE>\n")
	buffer.WriteString("        >\n")
	buffer.WriteString("    ];\n")
	buffer.WriteString("}\n")

	// Guardar el archivo DOT
	dotFile := OutPutPath + ".dot"
	if err := os.WriteFile(dotFile, []byte(buffer.String()), 0664); err != nil {
		return fmt.Errorf("no se pudo escribir el archivo DOT: %v", err)
	}
	// Ejecutar Graphviz para generar la imagen (PNG)
	cmd := exec.Command("dot", "-Tpng", "-Gdpi=300", dotFile, "-o", OutPutPath+".png")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error al ejecutar dot: %v", err)
	}
	OutPut.Println("Reporte de superbloque generado en:", OutPutPath+".png")
	return nil
}

// DiskExplorerResponse representa un nodo en el árbol del disco
type DiskExplorerResponse struct {
	Name     string                 `json:"name"`
	Type     string                 `json:"type"` // "folder" o "file"
	Children []DiskExplorerResponse `json:"children,omitempty"`
}

func ExploreDisk(id string) ([]DiskExplorerResponse, error) {
	partitionPath := GetPartitionPathByID(id)
	if partitionPath == "" {
		return nil, fmt.Errorf("no se encontró la ruta para el id: %s", id)
	}
	file, err := Utilities.OpenFile(partitionPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	partitionStart := GetPartitionStartByID(id)
	if partitionStart < 0 {
		return nil, fmt.Errorf("no se encontró la partición con el id %s", id)
	}

	var sb Structs.Superblock
	if err := Utilities.ReadObject(file, &sb, partitionStart); err != nil {
		return nil, err
	}

	nodo, err := exploreInodeTree(0, "/", file, sb)
	if err != nil {
		return nil, err
	}
	return []DiskExplorerResponse{nodo}, nil
}

// exploreInodeTree recorre recursivamente el árbol de inodos y construye la estructura JSON
func exploreInodeTree(inodeIndex int, name string, file *os.File, sb Structs.Superblock) (DiskExplorerResponse, error) {
	inode, _ := GetInodeFromIndex(inodeIndex, file, sb)
	if inode == nil {
		return DiskExplorerResponse{}, fmt.Errorf("no se pudo leer el inodo %d", inodeIndex)
	}

	nodo := DiskExplorerResponse{
		Name: name,
		Type: "file",
	}

	if inode.I_type[0] == '0' {
		nodo.Type = "folder"
		nodo.Children = []DiskExplorerResponse{}

		for _, blockIndex := range inode.I_block {
			if blockIndex == -1 {
				continue
			}
			folder, err := ReadFolderBlock(file, sb, blockIndex)
			if err != nil {
				continue
			}
			for _, entry := range folder.B_content {
				childName := strings.Trim(string(entry.B_name[:]), "\x00")
				if childName == "" || childName == "." || childName == ".." {
					continue
				}
				childIndex := int(entry.B_inodo)
				childNode, err := exploreInodeTree(childIndex, childName, file, sb)
				if err != nil {
					continue
				}
				nodo.Children = append(nodo.Children, childNode)
			}
		}
	}

	return nodo, nil
}

// Función auxiliar: obtiene un inodo por índice
func GetInodeFromIndex(index int, file *os.File, sb Structs.Superblock) (*Structs.Inode, int64) {
	inodeSize := binary.Size(Structs.Inode{})
	offset := int64(sb.S_inode_start) + int64(index)*int64(inodeSize)
	var inode Structs.Inode
	if err := Utilities.ReadObject(file, &inode, offset); err != nil {
		return nil, 0
	}
	return &inode, offset
}

// Función auxiliar: lee un FolderBlock dado el índice de bloque
func ReadFolderBlock(file *os.File, sb Structs.Superblock, blockIndex int32) (*Structs.Folderblock, error) {
	if blockIndex == -1 {
		return nil, fmt.Errorf("blockIndex=-1, no hay folder que leer")
	}
	blockSize := binary.Size(Structs.Folderblock{})
	offset := int64(sb.S_block_start) + int64(blockIndex)*int64(blockSize)
	var folder Structs.Folderblock
	if err := Utilities.ReadObject(file, &folder, offset); err != nil {
		return nil, err
	}
	return &folder, nil
}

func GetPartitionsByDisk(diskName string) []map[string]interface{} {
	mounted := GetMountedPartitions()
	var result []map[string]interface{}

	for disk, parts := range mounted {
		if filepath.Base(disk) == diskName {
			// Abrir el archivo del disco y leer el MBR
			diskPath := "./tets/" + diskName + ".dsk"
			file, err := Utilities.OpenFile(diskPath)
			if err != nil {
				continue
			}
			var mbr Structs.MRB
			if err := Utilities.ReadObject(file, &mbr, 0); err != nil {
				file.Close()
				continue
			}
			file.Close()
			for _, p := range parts {
				// Buscar la partición real en el MBR por ID
				var tipo, status string
				var size int64
				for _, realPart := range mbr.Partitions {
					if strings.Trim(string(realPart.Id[:]), "\x00") == p.ID {
						tipo = string(realPart.Type[:])
						status = string(realPart.Status[:])
						size = realPart.Size
						break
					}
				}
				result = append(result, map[string]interface{}{
					"name":     strings.Trim(string(p.Name[:]), "\x00"),
					"id":       p.ID,
					"type":     tipo,
					"status":   status,
					"start":    p.Start,
					"size":     size,
					"loggedIn": p.LoggedIn,
				})
			}
		}
	}

	return result
}

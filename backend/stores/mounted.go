package stores

import (
	"MIA_P1/Structs"
	"MIA_P1/Utilities"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"MIA_P1/OutPut"
)

// Carnet de estudiante (últimos dos dígitos)
const Carnet string = "00" 

// GetMountedPartition obtiene la partición montada con el id especificado
func GetMountedPartition(id string) (*Structs.Partition, string, error) {
	// Buscar el disco que contiene la partición con el id
	diskPath, err := findDiskPathByPartitionID(id)
	if err != nil {
		return nil, "", err
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return nil, "", fmt.Errorf("error al abrir el archivo del disco: %v", err)
	}
	defer file.Close()

	// Leer el MBR
	var mbr Structs.MRB
	if err := Utilities.ReadObject(file, &mbr, 0); err != nil {
		return nil, "", fmt.Errorf("error al leer el MBR: %v", err)
	}

	// Buscar la partición con el id especificado
	for i := 0; i < 4; i++ {
		partitionID := strings.Trim(string(mbr.Partitions[i].Id[:]), "\x00")
		if partitionID == id && mbr.Partitions[i].Size != 0 {
			return &mbr.Partitions[i], diskPath, nil
		}
	}

	return nil, "", errors.New("la partición no está montada")
}

// GetMountedMBR obtiene el MBR de la partición montada con el id especificado
func GetMountedMBR(id string) (*Structs.MRB, string, error) {
	// Buscar el disco que contiene la partición con el id
	diskPath, err := findDiskPathByPartitionID(id)
	if err != nil {
		return nil, "", err
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return nil, "", fmt.Errorf("error al abrir el archivo del disco: %v", err)
	}
	defer file.Close()

	// Leer el MBR
	var mbr Structs.MRB
	if err := Utilities.ReadObject(file, &mbr, 0); err != nil {
		return nil, "", fmt.Errorf("error al leer el MBR: %v", err)
	}

	// Verificar que la partición existe
	for i := 0; i < 4; i++ {
		if strings.Trim(string(mbr.Partitions[i].Id[:]), "\x00") == id && mbr.Partitions[i].Size != 0 {
			return &mbr, diskPath, nil
		}
	}

	return nil, "", errors.New("la partición no está montada")
}

// GetMountedPartitionSuperblock obtiene el Superblock de la partición montada
func GetMountedPartitionSuperblock(id string) (*Structs.Superblock, *Structs.Partition, string, error) {
	// Obtener la partición y el path del disco
	partition, diskPath, err := GetMountedPartition(id)
	if err != nil {
		return nil, nil, "", err
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("error al abrir el archivo del disco: %v", err)
	}
	defer file.Close()

	// Leer el Superblock
	var superblock Structs.Superblock
	if err := Utilities.ReadObject(file, &superblock, int64(partition.Start)); err != nil {
		return nil, nil, "", fmt.Errorf("error al leer el Superblock: %v", err)
	}

	return &superblock, partition, diskPath, nil
}

// GetPartitions obtiene todas las particiones de un disco
func GetPartitions(diskPath string) ([]Structs.Partition, error) {
	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return nil, fmt.Errorf("error al abrir el archivo del disco: %v", err)
	}
	defer file.Close()

	// Leer el MBR
	var mbr Structs.MRB
	if err := Utilities.ReadObject(file, &mbr, 0); err != nil {
		return nil, fmt.Errorf("error al leer el MBR: %v", err)
	}

	// Devolver las particiones
	partitions := make([]Structs.Partition, 4)
	for i := 0; i < 4; i++ {
		partitions[i] = mbr.Partitions[i]
	}
	return partitions, nil
}

// LoadMBR carga el MBR desde un archivo binario
func LoadMBR(diskPath string) (*Structs.MRB, error) {
	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return nil, fmt.Errorf("error al abrir el archivo del disco: %v", err)
	}
	defer file.Close()

	// Leer el MBR
	var mbr Structs.MRB
	if err := Utilities.ReadObject(file, &mbr, 0); err != nil {
		return nil, fmt.Errorf("error al leer el MBR: %v", err)
	}

	return &mbr, nil
}

// ListMountedPartitions lista todas las particiones montadas
func ListMountedPartitions() {
	OutPut.Println("======Mounted Partitions======")
	
	// Obtener todos los discos en la carpeta base
	basePath := "./tets" // Changed from /home/user/MIA/P1
	files, err := os.ReadDir(basePath)
	if err != nil {
		OutPut.Println("Error al leer la carpeta de discos:", err)
		return
	}

	// Iterar sobre los archivos .dsk
	count := 0
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".dsk" {
			diskPath := filepath.Join(basePath, file.Name())
			partitions, err := GetPartitions(diskPath)
			if err != nil {
				OutPut.Println("Error al leer particiones de", diskPath, ":", err)
				continue
			}
			for _, partition := range partitions {
				if partition.Size != 0 && string(partition.Status[:]) == "1" {
					id := strings.Trim(string(partition.Id[:]), "\x00")
					name := strings.Trim(string(partition.Name[:]), "\x00")
					fmt.Printf("ID: %s, Name: %s, Disk: %s\n", id, name, file.Name())
					count++
				}
			}
		}
	}

	if count == 0 {
		OutPut.Println("No hay particiones montadas")
	}
	OutPut.Println("======End Mounted Partitions======")
}

// findDiskPathByPartitionID busca el disco que contiene una partición con el id especificado
func findDiskPathByPartitionID(id string) (string, error) {
	basePath := "./tets"
	files, err := os.ReadDir(basePath)
	if err != nil {
		return "", fmt.Errorf("error al leer la carpeta de discos: %v", err)
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no hay discos en %s", basePath)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".dsk" {
			diskPath := filepath.Join(basePath, file.Name())
			partitions, err := GetPartitions(diskPath)
			if err != nil {
				fmt.Printf("Error reading partitions from %s: %v\n", diskPath, err)
				continue
			}
			for _, partition := range partitions {
				partitionID := strings.Trim(string(partition.Id[:]), "\x00")
				if partitionID == id && partition.Size != 0 {
					fmt.Printf("Found partition ID %s in disk %s\n", id, diskPath)
					return diskPath, nil
				}
			}
		}
	}

	return "", fmt.Errorf("la partición con ID %s no está montada", id)
}
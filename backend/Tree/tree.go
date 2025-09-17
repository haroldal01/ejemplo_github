package Tree

import (
	"MIA_P1/Structs"
	"MIA_P1/DiskManagement"
	"MIA_P1/Utilities"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func TreeReport(id string, outputPath string) error {
	// 1. Obtener la ruta del disco a partir del ID
	partitionPath := DiskManagement.GetPartitionPathByID(id)
	if partitionPath == "" {
		return fmt.Errorf("no se encontró la ruta para el id: %s", id)
	}

	// 2. Abrir archivo del disco
	file, err := Utilities.OpenFile(partitionPath)
	if err != nil {
		return fmt.Errorf("no se pudo abrir el disco: %v", err)
	}
	defer file.Close()

	// 3. Obtener el inicio de la partición y leer el Superblock
	partitionStart := DiskManagement.GetPartitionStartByID(id)
	if partitionStart < 0 {
		return fmt.Errorf("no se encontró la partición con el id %s", id)
	}

	var sb Structs.Superblock
	if err := Utilities.ReadObject(file, &sb, int64(partitionStart)); err != nil {
		return fmt.Errorf("error al leer el Superblock: %v", err)
	}

	// 4. Construir archivo DOT
	var buffer strings.Builder
	buffer.WriteString("digraph ext2_tree {\n")
	buffer.WriteString("    rankdir=LR;\n")
	buffer.WriteString("    node [fontname=\"Arial\", shape=plain, style=\"filled\", fontsize=10];\n")
	buffer.WriteString("    edge [color=\"#5A5A5A\", arrowhead=normal];\n")
	buffer.WriteString("    graph [bgcolor=\"#ffffff\", pencolor=\"#333333\", penwidth=2.0, style=\"rounded\"];\n\n")

	// 5. Recorrer el árbol desde el inodo raíz
	visited := make(map[int]bool)
	if err := traverseInodeTree(0, "root", file, sb, &buffer, visited); err != nil {
		return fmt.Errorf("error al recorrer el árbol de inodos: %v", err)
	}

	buffer.WriteString("}\n")

	// 6. Guardar archivo .dot
	dotFile := outputPath + ".dot"
	if err := os.WriteFile(dotFile, []byte(buffer.String()), 0644); err != nil {
		return fmt.Errorf("error al guardar archivo DOT: %v", err)
	}

	// 7. Generar imagen con Graphviz
	cmd := exec.Command("dot", "-Tpng", "-Gdpi=300", dotFile, "-o", outputPath+".png")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error al generar imagen PNG: %v", err)
	}

	return nil
}


func traverseInodeTree(inodeIndex int, label string, file *os.File, sb Structs.Superblock, buffer *strings.Builder, visited map[int]bool) error {
	if visited[inodeIndex] {
		// Ya fue procesado este inodo, evitar bucle
		return nil
	}
	visited[inodeIndex] = true

	// 1. Leer el inodo desde disco
	inode, _ := GetInodeFromIndex(inodeIndex, file, sb)
	if inode == nil {
		return fmt.Errorf("no se pudo leer el inodo %d", inodeIndex)
	}

	// 2. Crear nodo en DOT con información del inodo
	nodeName := fmt.Sprintf("inode%d", inodeIndex)
	addInodeNode(buffer, nodeName, inodeIndex, *inode, label)

	// 3. Verificar si es carpeta (I_type[0] == '0')
	if inode.I_type[0] == '0' {
		// Leer el FolderBlock principal (asumiendo que I_block[0] es el bloque de carpeta)
		folderBlockIndex := inode.I_block[0]
		if folderBlockIndex != -1 {
			folder, err := ReadFolderBlock(file, sb, folderBlockIndex)
			if err != nil {
				return fmt.Errorf("error al leer folderblock del inodo %d: %v", inodeIndex, err)
			}
			// Recorrer entradas (omitir "." y "..")
			for _, entry := range folder.B_content {
				name := strings.Trim(string(entry.B_name[:]), "\x00")
				if name == "" || name == "." || name == ".." {
					continue
				}
				childIndex := int(entry.B_inodo)
				// Agregar arista (inode -> child)
				childNodeName := fmt.Sprintf("inode%d", childIndex)
				buffer.WriteString(fmt.Sprintf("    %s -> %s;\n", nodeName, childNodeName))
				// Recursión
				if err := traverseInodeTree(childIndex, name, file, sb, buffer, visited); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func addInodeNode(buffer *strings.Builder, nodeName string, inodeIndex int, inode Structs.Inode, label string) {
	// Construir cadena de bloques asignados
	blocksStr := ""
	for j, b := range inode.I_block {
		if b != -1 {
			blocksStr += fmt.Sprintf("B%d=%d\\n", j, b)
		}
	}
	atime := strings.Trim(string(inode.I_atime[:]), "\x00")
	ctime := strings.Trim(string(inode.I_ctime[:]), "\x00")
	mtime := strings.Trim(string(inode.I_mtime[:]), "\x00")
	perm := strings.Trim(string(inode.I_perm[:]), "\x00")

	// Tipo de inodo: '0' -> directorio, '1' -> archivo
	var inodeType string
	if inode.I_type[0] == '0' {
		inodeType = "Directorio"
	} else {
		inodeType = "Archivo"
	}

	buffer.WriteString(fmt.Sprintf("    %s [\n", nodeName))
	buffer.WriteString("        fillcolor=\"#F9F3C2\",\n")
	buffer.WriteString("        label=<\n")
	buffer.WriteString("            <TABLE BORDER=\"0\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\" BGCOLOR=\"#FFFEEB\">\n")
	buffer.WriteString(fmt.Sprintf("                <TR><TD COLSPAN=\"2\" BGCOLOR=\"#F7DF72\"><B>%s (Inodo %d)</B></TD></TR>\n", label, inodeIndex))
	buffer.WriteString(fmt.Sprintf("                <TR><TD><B>Tipo</B></TD><TD>%s</TD></TR>\n", inodeType))
	buffer.WriteString(fmt.Sprintf("                <TR><TD><B>UID</B></TD><TD>%d</TD></TR>\n", inode.I_uid))
	buffer.WriteString(fmt.Sprintf("                <TR><TD><B>GID</B></TD><TD>%d</TD></TR>\n", inode.I_gid))
	buffer.WriteString(fmt.Sprintf("                <TR><TD><B>Tamaño</B></TD><TD>%d bytes</TD></TR>\n", inode.I_size))
	buffer.WriteString(fmt.Sprintf("                <TR><TD><B>Permisos</B></TD><TD>%s</TD></TR>\n", perm))
	buffer.WriteString(fmt.Sprintf("                <TR><TD><B>ATime</B></TD><TD>%s</TD></TR>\n", atime))
	buffer.WriteString(fmt.Sprintf("                <TR><TD><B>CTime</B></TD><TD>%s</TD></TR>\n", ctime))
	buffer.WriteString(fmt.Sprintf("                <TR><TD><B>MTime</B></TD><TD>%s</TD></TR>\n", mtime))
	buffer.WriteString(fmt.Sprintf("                <TR><TD><B>Bloques</B></TD><TD>%s</TD></TR>\n", blocksStr))
	buffer.WriteString("            </TABLE>\n")
	buffer.WriteString("        >\n")
	buffer.WriteString("    ];\n")
}

func GetInodeFromIndex(index int, file *os.File, sb Structs.Superblock) (*Structs.Inode, int64) {
	inodeSize := binary.Size(Structs.Inode{})
	offset := int64(sb.S_inode_start) + int64(index)*int64(inodeSize)
	var inode Structs.Inode
	if err := Utilities.ReadObject(file, &inode, offset); err != nil {
		return nil, 0
	}
	return &inode, offset
}

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

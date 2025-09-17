package Analyzer

import (
	"MIA_P1/DiskManagement"
	"MIA_P1/OutPut"
	"MIA_P1/Structs"
	"MIA_P1/Tree"
	"MIA_P1/UserManager"
	"MIA_P1/Utilities"
	"MIA_P1/stores"
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var re = regexp.MustCompile(`-(\w+)=("[^"]+"|\S+)`)

func fn_execute(params string) {
	fs := flag.NewFlagSet("execute", flag.ContinueOnError)
	path := fs.String("path", "", "Ruta del archivo script")

	if err := fs.Parse(strings.Fields(params)); err != nil {
		OutPut.Println("Error al parsear parámetros:", err)
		return
	}
	if *path == "" {
		OutPut.Println("Error: el parámetro -path es obligatorio")
		return
	}

	normalizedPath := strings.ReplaceAll(*path, "\\", "/")
	normalizedPath = strings.Trim(normalizedPath, "\"") // Elimina comillas

	if !strings.HasSuffix(strings.ToLower(normalizedPath), ".sdaa") {
		OutPut.Println("Error: el archivo debe tener la extensión .sdaa")
		return
	}

	// Lee el contenido del archivo
	content, err := os.ReadFile(normalizedPath)
	if err != nil {
		fmt.Printf("Error al leer el archivo %s: %v\n", *path, err)
		return
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			continue // línea vacía
		}

		if strings.HasPrefix(line, "#") {
			OutPut.Println(line) // muestra comentarios
			continue
		}

		// Ejecuta comando
		fmt.Printf(">> %s\n", line)
		command, params := GetCommandAndParams(line)
		AnalyzeCommand(command, params)
	}
}

func Analyze() {
	scanner := bufio.NewScanner(os.Stdin)
	OutPut.Println("Ingrese un comando (o 'exit' para salir):")
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "" || strings.HasPrefix(input, "#") {
			continue
		}
		command, params := GetCommandAndParams(input)
		OutPut.Println("Command:", command, "Params:", params)
		AnalyzeCommand(command, params)
		OutPut.Println("Ingrese un comando (o 'exit' para salir):")
	}
	if err := scanner.Err(); err != nil {
		OutPut.Println("Error reading input:", err)
	}
}

func GetCommandAndParams(input string) (string, string) {
	parts := strings.Fields(input)
	if len(parts) > 0 {
		command := strings.ToLower(parts[0])
		params := strings.Join(parts[1:], " ")
		return command, params
	}
	return "", input
}

func fn_pause() string {
	return "[PAUSE]"
}

func fn_mount(params string) {
	fs := flag.NewFlagSet("mount", flag.ExitOnError)
	name := fs.String("driveletter", "", "Letter of the drive")
	path := fs.String("name", "", "name of the partition")
	managementFlags(fs, params)
	DiskManagement.Mount(*name, *path)
}

func fn_fdisk(params string) {
	fs := flag.NewFlagSet("fdisk", flag.ExitOnError)
	size := fs.Int("size", 0, "Size")
	driveLetter := fs.String("driveletter", "", "Letra de unidad")
	name := fs.String("name", "", "Nombre de la partición")
	type_ := fs.String("type", "P", "Tipo de partición (p/e)")
	fit := fs.String("fit", "F", "Fit")
	delete := fs.String("delete", "", "Delete")
	unit := fs.String("unit", "M", "Unit")
	add := fs.Int("add", 0, "Add")
	managementFlags(fs, params)
	DiskManagement.Fdisk(*size, strings.ToUpper(*driveLetter), *name,
		strings.ToUpper(*type_), strings.ToUpper(*fit), strings.ToUpper(*delete), strings.ToUpper(*unit), *add)
}

func fn_mkdisk(params string) {
	fs := flag.NewFlagSet("mkdisk", flag.ExitOnError)
	size := fs.Int("size", 0, "Size")
	fit := fs.String("fit", "FF", "Fit")
	unit := fs.String("unit", "M", "Unit")
	managementFlags(fs, params)
	DiskManagement.Mkdisk(*size, strings.ToUpper(*fit), strings.ToUpper(*unit))
}

func fn_rmdisk(params string) string {
	fs := flag.NewFlagSet("rmdisk", flag.ExitOnError)
	driveLetter := fs.String("driveletter", "", "Drive letter")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	managementFlags(fs, params)
	return DiskManagement.Rmdisk(strings.ToUpper(*driveLetter), *confirm)
}

func managementFlags(fs *flag.FlagSet, params string) {
	fs.Parse(os.Args[1:])
	matches := re.FindAllStringSubmatch(params, -1)
	var flagNames []string
	fs.VisitAll(func(f *flag.Flag) {
		flagNames = append(flagNames, f.Name)
	})
	for _, match := range matches {
		flagName := match[1]
		flagValue := match[2]
		flagValue = strings.Trim(flagValue, "\"")
		if contains(flagNames, flagName) {
			fs.Set(flagName, flagValue)
		} else {
			OutPut.Println("Error: Flag not found:", flagName)
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func fn_mkfs(params string) {
	fs := flag.NewFlagSet("mkfs", flag.ExitOnError)
	id := fs.String("id", "", "Partition ID")
	type_ := fs.String("type", "FULL", "Format type")
	fsType := fs.String("fs", "2FS", "Filesystem (2FS/3FS)")
	managementFlags(fs, params)
	if err := UserManager.Mkfs(strings.ToUpper(*id), strings.ToUpper(*type_), strings.ToUpper(*fsType)); err != nil {
		OutPut.Println("Error:", err)
	}
}

func fn_unmount(params string) {
	fs := flag.NewFlagSet("unmount", flag.ExitOnError)
	id := fs.String("id", "", "Partition ID")
	managementFlags(fs, params)
	DiskManagement.Unmount(strings.ToUpper(*id))
}

func fn_login(params string) {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	user := fs.String("user", "", "Usuario")
	pass := fs.String("pass", "", "Contraseña")
	id := fs.String("id", "", "ID de partición")

	if err := fs.Parse(strings.Fields(params)); err != nil {
		OutPut.Println("Error al parsear parámetros:", err)
		return
	}

	if *user == "" || *pass == "" || *id == "" {
		OutPut.Println("Error: todos los parámetros son obligatorios")
		return
	}

	if err := UserManager.Login(*user, *pass, *id); err != nil {
		OutPut.Println("ADVERTENCIA:", err.Error())
	}
}

func fn_logout(params string) {
	if params != "" {
		OutPut.Println("Error: logout no acepta parámetros")
		return
	}
	if err := UserManager.Logout(); err != nil {
		OutPut.Println("Error:", err)
	}
}

func fn_mkgrp(params string) string {
	fs := flag.NewFlagSet("mkgrp", flag.ContinueOnError)
	name := fs.String("name", "", "Name")
	params = strings.ToLower(params)
	if err := fs.Parse(strings.Fields(params)); err != nil {
		return "Error al parsear los paràmetros: " + err.Error()
	}

	if *name == "" {
		return "Error: el nombre esta vacio"
	}

	err := UserManager.Mkgrp(*name)
	if err != nil {
		OutPut.Println("Error al crear el grupo:", err)
		return "Error al crear el grupo: " + err.Error()
	}
	return "Grupo creado correctamente"
}

func fn_rmgrp(params string) {
	fs := flag.NewFlagSet("rmgrp", flag.ContinueOnError)
	name := fs.String("name", "", "Nombre del grupo a eliminar")
	params = strings.ToLower(params)

	if err := fs.Parse(strings.Fields(params)); err != nil {
		OutPut.Println("Error al parsear los parámetros:", err)
		return
	}

	if *name == "" {
		OutPut.Println("Error: el nombre del grupo no puede estar vacío")
		return
	}

	err := UserManager.Rmgrp(*name)
	if err != nil {
		OutPut.Println("ADVERTENCIA:", err.Error())
		return
	}

	OutPut.Println("Grupo eliminado correctamente")
}

func fn_mkusr(params string) {
	fs := flag.NewFlagSet("mkusr", flag.ContinueOnError)
	user := fs.String("user", "", "Nombre del usuario")
	pass := fs.String("pass", "", "Contraseña")
	grp := fs.String("grp", "", "Grupo")

	if err := fs.Parse(strings.Fields(params)); err != nil {
		OutPut.Println("Error al parsear los parámetros:", err)
		return
	}

	if *user == "" || *pass == "" || *grp == "" {
		OutPut.Println("Error: todos los parámetros son obligatorios")
		return
	}

	if err := UserManager.Mkusr(*user, *pass, *grp); err != nil {
		OutPut.Println("ADVERTENCIA:", err.Error())
	} else {
		OutPut.Println("Usuario creado correctamente.")
	}
}

func fn_rmusr(params string) {
	fs := flag.NewFlagSet("rmusr", flag.ContinueOnError)
	user := fs.String("user", "", "Nombre del usuario a eliminar")

	if err := fs.Parse(strings.Fields(params)); err != nil {
		OutPut.Println("Error al parsear los parámetros:", err)
		return
	}

	if *user == "" {
		OutPut.Println("Error: el nombre del usuario es obligatorio")
		return
	}

	if err := UserManager.Rmusr(*user); err != nil {
		OutPut.Println("ADVERTENCIA:", err.Error())
	} else {
		OutPut.Println("Usuario eliminado correctamente.")
	}
}

func fn_mkfile(params string) {
	fs := flag.NewFlagSet("mkfile", flag.ContinueOnError)

	path := fs.String("path", "", "Ruta del archivo a crear")
	size := fs.Int("size", 0, "Tamaño del archivo en bytes")
	cont := fs.String("cont", "", "Ruta a un archivo externo con contenido")
	createParents := fs.Bool("r", false, "Crear carpetas padre si no existen")

	if err := fs.Parse(strings.Fields(params)); err != nil {
		OutPut.Println("Error al parsear los parámetros:", err)
		return
	}

	// Validaciones
	if *path == "" {
		OutPut.Println("Error: El parámetro -path es obligatorio")
		return
	}

	if *size < 0 {
		OutPut.Println("Error: El tamaño no puede ser negativo")
		return
	}

	if *cont != "" && *size > 0 {
		OutPut.Println("Advertencia: Se usará el archivo de contenido. El parámetro -size será ignorado.")
	}

	result := UserManager.Mkfile(*path, *createParents, *size, *cont)

	if result != "" {
		OutPut.Println("ADVERTENCIA:", result)
	} else {
		OutPut.Println("Archivo creado correctamente.")
	}
}

func fn_cat(params string) {
	// Parse the parameters string to extract fileN parameters
	matches := re.FindAllStringSubmatch(params, -1)
	fileParams := make(map[string]string)

	// Convert the parameters to the expected format
	for _, match := range matches {
		flagName := match[1]
		flagValue := strings.Trim(match[2], "\"")
		if strings.HasPrefix(flagName, "file") {
			fileParams[flagName] = flagValue
		}
	}

	// Call the Cat function from UserManager
	result := UserManager.Cat(fileParams)

	// Print the result
	OutPut.Println(result)
}

func fn_mkdir(params string) {
	fs := flag.NewFlagSet("mkdir", flag.ContinueOnError)
	path := fs.String("path", "", "Path of the directory to create")
	createParents := fs.Bool("r", false, "Create parent directories if they don't exist")

	// Parse parameters
	managementFlags(fs, params)

	if *path == "" {
		OutPut.Println("Error: El parámetro -path es obligatorio")
		return
	}

	// Call Mkdir from UserManager
	result := UserManager.Mkdir(*path, *createParents)
	OutPut.Println(result)
}

func fn_reportMBR(params string) {
	fs := flag.NewFlagSet("rep", flag.ExitOnError)
	name := fs.String("name", "", "Nombre del reporte")
	path := fs.String("path", "", "Ruta del archivo de salida")
	id := fs.String("id", "", "ID de la partición montada")
	managementFlags(fs, params)

	// Validaciones básicas
	if strings.ToLower(*name) != "mbr" {
		OutPut.Println("Error: Este handler solo soporta el reporte 'mbr'")
		return
	}
	if *path == "" || *id == "" {
		OutPut.Println("Error: Debe especificar -path y -id")
		return
	}

	// Buscar la partición montada con ese ID
	partition, diskPath, err := stores.GetMountedPartition(*id)
	if err != nil {
		OutPut.Println("Error:", err)
		return
	}
	if partition == nil {
		OutPut.Println("Error: No se encontró la partición con ID", *id)
		return
	}

	// Abrir archivo y leer MBR
	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		OutPut.Println("Error al abrir el disco:", err)
		return
	}
	defer file.Close()

	var mbr Structs.MRB
	if err := Utilities.ReadObject(file, &mbr, 0); err != nil {
		OutPut.Println("Error al leer el MBR:", err)
		return
	}

	// Llamar a la función que genera el reporte
	if err := DiskManagement.ReportMBR(&mbr, *path); err != nil {
		OutPut.Println("Error al generar el reporte MBR:", err)
	}
}

func generarReportes(params string) string {
	//se crea un nuevo conjunto de flags
	fs := flag.NewFlagSet("rep", flag.ContinueOnError)

	//parametros
	name := fs.String("name", "", "Nombre del reporte")
	path := fs.String("path", "", "Ruta del reporte")
	id := fs.String("id", "", "ID de la partición")
	path_file_ls := fs.String("path_file_ls", "", "Ruta del archivo de salida para el reporte file y ls")

	//params = strings.ToLower(params)

	//parsear los parametros
	args := strings.Split(params, " ")
	err := fs.Parse(args)
	if err != nil {
		OutPut.Println("Error al parsear los parámetros:", err)
		return "Error al parsear los parámetros:"
	}

	// Validar que los parámetros obligatorios estén presentes
	if *name == "" || *path == "" || *id == "" {
		OutPut.Println("Error: Los parámetros -name, -path e -id son obligatorios")
		return "Error: Los parámetros -name, -path e -id son obligatorios"
	}

	// Validar que el nombre del reporte sea uno de los valores permitidos
	validNames := map[string]bool{
		"mbr": true, "disk": true, "inode": true, "block": true,
		"bm_inode": true, "bm_block": true, "tree": true, "sb": true,
		"file": true, "ls": true,
	}

	if !validNames[*name] {

		return "Error: El valor de -name debe ser uno de los siguientes: mbr, disk, inode, block, bm_inode, bm_block, tree, sb, file, ls"
	}

	// Para reportes file y ls, validar que el parámetro path_file_ls esté presente
	if (*name == "file" || *name == "ls") && *path_file_ls == "" {
		OutPut.Println("Error: Para reportes file y ls, el parámetro -path_file_ls es obligatorio")
		return "Error: Para reportes file y ls, el parámetro -path_file_ls es obligatorio"
	}

	// Verificar que la partición con el ID especificado esté montada
	foundPartition := false

	for _, partitions := range DiskManagement.GetMountedPartitions() {
		for _, partition := range partitions {
			if partition.ID == *id {
				foundPartition = true
				break
			}
		}
		if foundPartition {
			break
		}
	}

	if !foundPartition {
		OutPut.Println("Error: No se encontró ninguna partición montada con el ID", *id)
		return "Error: No se encontró ninguna partición montada con el ID" + *id
	}

	// Crear la carpeta de destino si no existe
	dirPath := filepath.Dir(*path)
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		OutPut.Println("Error al crear la carpeta de destino:", err)
		return "Error al crear la carpeta de destino:"
	}

	switch *name {
	case "tree":
		Tree.TreeReport(*id, *path)
	case "mbr":
		fn_reportMBR(params)
	case "disk":
		DiskManagement.DiskReport(*id, *path)
	case "inode":
		DiskManagement.InodeReport(*id, *path)
	case "block":
		DiskManagement.BlockReport(*id, *path)
	case "bm_inode":
		DiskManagement.BmInodeReport(*id, *path)
	case "bm_block":
		DiskManagement.BmBlockReport(*id, *path)
	case "file":
		UserManager.ReportFile(*id, *path, *path_file_ls)
	case "ls":
		UserManager.ReportLs(*id, *path, *path_file_ls)
	case "sb":
		DiskManagement.SuperBlockReport(*id, *path)
	default:
		OutPut.Println("Error: El nombre del reporte no es válido")
	}
	return "Reporte generado correctamente: " + *path
}

func AnalyzeCommand(command string, params string) string {
	switch command {
	case "mkdisk":
		fn_mkdisk(params)
		return "Disco creado correctamente"
	case "fdisk":
		fn_fdisk(params)
		return "Partición gestionada correctamente"
	case "mount":
		fn_mount(params)
		return "Partición montada correctamente"
	case "rmdisk":
		return fn_rmdisk(params)
	case "pause":
		fn_pause()
		return "Pausa completada"
	case "mkfs":
		fn_mkfs(params)
		return "Sistema de archivos creado correctamente"
	case "listmount":
		stores.ListMountedPartitions()
		return "Particiones montadas listadas"
	case "unmount":
		fn_unmount(params)
		return "Partición desmontada correctamente"
	case "login":
		fn_login(params)
		return "Sesión iniciada"
	case "logout":
		fn_logout(params)
		return "Sesión cerrada"
	case "mkgrp":
		return fn_mkgrp(params)
	case "rmgrp":
		fn_rmgrp(params)
		return "Grupo eliminado correctamente"
	case "mkusr":
		fn_mkusr(params)
		return "Usuario creado correctamente"
	case "rmusr":
		fn_rmusr(params)
		return "Usuario eliminado correctamente"
	case "mkfile":
		fn_mkfile(params)
		return "Archivo creado correctamente"
	case "cat":
		fn_cat(params)
		return "Comando cat ejecutado"
	case "mkdir":
		fn_mkdir(params)
		return "Directorio creado correctamente"
	case "rep":
		return generarReportes(params)
	case "execute":
		fn_execute(params)
		return "Script ejecutado"
	case "exit":
		OutPut.Println("Exiting the program.")
		os.Exit(0)
		return "Saliendo del programa"
	default:
		return "Error: comando no reconocido."
	}
}

// ExecuteScript ejecuta una secuencia de comandos desde un string multilinea.
// Retorna los resultados, si está pausado, las líneas restantes del script, si requiere confirmación y el mensaje de confirmación.
func ExecuteScript(script string) ([]string, bool, []string, bool, string) {
	var results []string
	var remainingScriptLines []string
	lines := strings.Split(script, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		command, params := GetCommandAndParams(trimmed)

		// Si el comando es "pause", detener la ejecución
		if command == "pause" {
			results = append(results, fmt.Sprintf(">> %s\n", trimmed))
			results = append(results, "Presione continuar para seguir...")

			// Guardar las líneas restantes del script
			for j := i + 1; j < len(lines); j++ {
				remainingLine := strings.TrimSpace(lines[j])
				if remainingLine != "" && !strings.HasPrefix(remainingLine, "#") {
					remainingScriptLines = append(remainingScriptLines, lines[j])
				}
			}

			fmt.Println("DEBUG: Pausa detectada en ExecuteScript")
			return results, true, remainingScriptLines, false, ""
		}

		result := AnalyzeCommand(command, params)
		if strings.HasPrefix(result, "CONFIRM_RMDISK:") {
			results = append(results, fmt.Sprintf(">> %s\n", trimmed))
			// Guardar las líneas restantes del script
			for j := i + 1; j < len(lines); j++ {
				remainingLine := strings.TrimSpace(lines[j])
				if remainingLine != "" && !strings.HasPrefix(remainingLine, "#") {
					remainingScriptLines = append(remainingScriptLines, lines[j])
				}
			}
			fmt.Println("DEBUG: Confirmación de rmdisk detectada en ExecuteScript")
			return results, false, remainingScriptLines, true, result
		}

		results = append(results, fmt.Sprintf(">> %s\n", trimmed))
	}

	return results, false, nil, false, ""
}

// ExecuteScriptFromFile ejecuta un archivo .sdaa dado por el path.
func ExecuteScriptFromFile(param string) string {
	// Extraer -path=
	if !strings.Contains(param, "-path=") {
		return "Error: Falta el parámetro -path"
	}
	path := strings.Trim(strings.Split(param, "=")[1], "\" ")
	if !strings.HasSuffix(strings.ToLower(path), ".sdaa") {
		return "Error: el archivo debe tener la extensión .sdaa"
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("Error al leer el archivo: %v", err)
	}
	resultLines, _, _, _, _ := ExecuteScript(string(content))
	return strings.Join(resultLines, "\n")
}

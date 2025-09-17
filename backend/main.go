package main

import (
	"MIA_P1/Analyzer"
	"MIA_P1/DiskManagement"
	"MIA_P1/OutPut"
	"MIA_P1/UserManager"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// ---------- ESTRUCTURAS ----------
type LoginRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	PartitionID string `json:"partition_id"`
}

type ExecuteRequest struct {
	Input string `json:"input"`
}

type ExecuteScriptRequest struct {
	Script string `json:"script"`
}

type ContinueScriptRequest struct {
	RemainingScript []string `json:"remaining"`
}

type LoginResponse struct {
	Message string `json:"message"`
}

type LogoutResponse struct {
	Message string `json:"message"`
}

type SessionResponse struct {
	Message string `json:"message"`
}

type HealthResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
	Host      string `json:"host"`
}

type ExecuteResponse struct {
	Confirm bool   `json:"confirm,omitempty"`
	Message string `json:"message"`
	Console string `json:"console"`
}

type ExecuteScriptResponse struct {
	Confirm   bool     `json:"confirm,omitempty"`
	Message   string   `json:"message,omitempty"`
	Results   []string `json:"results"`
	Console   string   `json:"console"`
	Paused    bool     `json:"paused"`
	Remaining []string `json:"remaining,omitempty"`
}

type DisksResponse struct {
	Disks []DiskInfo `json:"disks"`
}

type DiskInfo struct {
	Name               string   `json:"name"`
	Path               string   `json:"path"`
	MountedPartitions  []string `json:"mounted_partitions"`
}

type AllDisksResponse struct {
	Disks []string `json:"disks"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// ---------- CONFIGURACIÓN ----------
func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return ":" + port
}

func getHost() string {
	host, _ := os.Hostname()
	return host
}

func getDiskDirectory() string {
	diskDir := os.Getenv("DISK_DIR")
	if diskDir == "" {
		diskDir = "./backend/tets/" // valor por defecto
	}
	return diskDir
}

// ---------- FUNCIÓN PRINCIPAL ----------
func main() {
	// Configuración de la aplicación Fiber para producción
	app := fiber.New(fiber.Config{
		// Configuraciones para producción
		Prefork:       false, // Para EC2, mejor en false
		CaseSensitive: true,
		StrictRouting: false,
		ServerHeader:  "MIA-Backend",
		AppName:       "MIA Project v1.0",
		
		// Timeouts apropiados para producción
		ReadTimeout:  time.Second * 30,
		WriteTimeout: time.Second * 30,
		IdleTimeout:  time.Second * 60,
		
		// Manejo de errores
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			
			log.Printf("Error: %v", err)
			return c.Status(code).JSON(ErrorResponse{
				Error: err.Error(),
			})
		},
	})

	fmt.Printf("Iniciando el backend en %s...\n", getHost())

	// Middleware para producción
	app.Use(recover.New()) // Recuperación de panics
	app.Use(logger.New(logger.Config{
		Format: "${time} ${status} - ${method} ${path} - ${ip} - ${latency}\n",
	}))

	// Configuración del middleware CORS para producción
	corsConfig := cors.Config{
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}
	
	// Configuración segura de CORS
	allowedOrigins := getAllowedOrigins()
	if allowedOrigins == "*" {
		// En desarrollo, permitir todo pero sin credentials
		corsConfig.AllowOrigins = "*"
		corsConfig.AllowCredentials = false
	} else {
		// En producción, usar orígenes específicos con credentials
		corsConfig.AllowOrigins = allowedOrigins
		corsConfig.AllowCredentials = true
	}
	
	app.Use(cors.New(corsConfig))

	// Endpoint para login
	app.Post("/login", handleLogin)

	// Endpoint para logout
	app.Post("/logout", handleLogout)

	// Endpoint para verificar el estado de la sesión
	app.Get("/session", handleSession)

	// Healthcheck endpoint mejorado para producción
	app.Get("/health", handleHealth)

	// API endpoints
	app.Post("/api/execute", handleExecute)
	app.Post("/api/executeScript", handleExecuteScript)
	app.Post("/api/continueScript", handleContinueScript)
	app.Get("/api/disk-tree/:id", handleDiskTree)
	app.Get("/api/partition-content/:id", handlePartitionContent)
	app.Get("/api/partitions/:disk", handlePartitionsByDisk)
	app.Get("/api/disks", handleDisks)
	app.Get("/disks/:name/partitions", handleDiskPartitions)
	app.Get("/api/test-partition/:id", handleTestPartition)
	app.Get("/api/all-disks", handleAllDisks)

	// Ruta para servir archivos estáticos si es necesario
	app.Static("/static", "./static")

	// Crear directorio de discos si no existe
	if err := os.MkdirAll(getDiskDirectory(), 0755); err != nil {
		log.Printf("Warning: No se pudo crear directorio de discos: %v", err)
	}

	// Iniciar el servidor
	port := getPort()
	log.Printf("Servidor iniciado en http://0.0.0.0%s", port)
	log.Printf("Host: %s", getHost())
	log.Printf("Directorio de discos: %s", getDiskDirectory())
	
	log.Fatal(app.Listen("0.0.0.0" + port)) // Escuchar en todas las interfaces
}

// ---------- CONFIGURACIONES AUXILIARES ----------

func getAllowedOrigins() string {
	origins := os.Getenv("ALLOWED_ORIGINS")
	if origins == "" {
		// En desarrollo permite todo, en producción deberías especificar dominios exactos
		return "*"
	}
	return origins
}

// ---------- HANDLERS ----------

func handleLogin(c *fiber.Ctx) error {
	var request LoginRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Datos inválidos",
		})
	}

	// Validar que todos los campos estén presentes
	if request.Username == "" || request.Password == "" || request.PartitionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Todos los campos son obligatorios",
		})
	}

	// Intentar hacer login usando UserManager
	err := UserManager.Login(request.Username, request.Password, request.PartitionID)
	if err != nil {
		log.Printf("Login fallido para usuario %s: %v", request.Username, err)
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	log.Printf("Login exitoso para usuario: %s", request.Username)
	return c.JSON(LoginResponse{
		Message: "Login exitoso",
	})
}

func handleLogout(c *fiber.Ctx) error {
	err := UserManager.Logout()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	log.Println("Logout exitoso")
	return c.JSON(LogoutResponse{
		Message: "Logout exitoso",
	})
}

func handleSession(c *fiber.Ctx) error {
	return c.JSON(SessionResponse{
		Message: "Endpoint de sesión disponible",
	})
}

func handleHealth(c *fiber.Ctx) error {
	return c.JSON(HealthResponse{
		Status:    "ok",
		Message:   "Backend funcionando correctamente",
		Timestamp: fmt.Sprintf("%d", time.Now().Unix()),
		Version:   "1.0.0",
		Host:      getHost(),
	})
}

func handleExecute(c *fiber.Ctx) error {
	var request ExecuteRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Datos inválidos",
		})
	}

	OutPut.Clear()
	command, params := Analyzer.GetCommandAndParams(request.Input)
	result := Analyzer.AnalyzeCommand(command, params)
	output := OutPut.GetOutput()

	log.Printf("Comando ejecutado: %s", request.Input)

	if strings.HasPrefix(result, "CONFIRM_RMDISK:") {
		return c.JSON(ExecuteResponse{
			Confirm: true,
			Message: result,
			Console: output,
		})
	}

	return c.JSON(ExecuteResponse{
		Message: result,
		Console: output,
	})
}

func handleExecuteScript(c *fiber.Ctx) error {
	var request ExecuteScriptRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Datos inválidos",
		})
	}

	OutPut.Clear()
	results, paused, remainingScriptLines, confirm, confirmMsg := Analyzer.ExecuteScript(request.Script)
	output := OutPut.GetOutput()

	log.Printf("Script ejecutado con %d líneas", len(strings.Split(request.Script, "\n")))

	if confirm {
		log.Println("DEBUG: Enviando confirmación de rmdisk al frontend desde /api/executeScript")
		return c.JSON(ExecuteScriptResponse{
			Confirm:   true,
			Message:   confirmMsg,
			Results:   results,
			Console:   output,
			Remaining: remainingScriptLines,
		})
	}

	if paused {
		return c.JSON(ExecuteScriptResponse{
			Results:   results,
			Console:   output,
			Paused:    true,
			Remaining: remainingScriptLines,
		})
	}

	return c.JSON(ExecuteScriptResponse{
		Results: results,
		Console: output,
		Paused:  false,
	})
}

func handleContinueScript(c *fiber.Ctx) error {
	var request ContinueScriptRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Datos inválidos",
		})
	}

	remainingScript := strings.Join(request.RemainingScript, "\n")
	OutPut.Clear()
	results, paused, remainingScriptLines, confirm, confirmMsg := Analyzer.ExecuteScript(remainingScript)
	output := OutPut.GetOutput()

	log.Println("Continuando script pausado")

	if confirm {
		log.Println("DEBUG: Enviando confirmación de rmdisk al frontend desde /api/continueScript")
		return c.JSON(ExecuteScriptResponse{
			Confirm:   true,
			Message:   confirmMsg,
			Results:   results,
			Console:   output,
			Remaining: remainingScriptLines,
		})
	}

	if paused {
		return c.JSON(ExecuteScriptResponse{
			Results:   results,
			Console:   output,
			Paused:    true,
			Remaining: remainingScriptLines,
		})
	}

	return c.JSON(ExecuteScriptResponse{
		Results: results,
		Console: output,
		Paused:  false,
	})
}

func handleDiskTree(c *fiber.Ctx) error {
	id := c.Params("id")
	tree, err := DiskManagement.ExploreDisk(id)
	if err != nil {
		log.Printf("Error explorando disco %s: %v", id, err)
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}
	return c.JSON(tree)
}

func handlePartitionContent(c *fiber.Ctx) error {
	id := c.Params("id")
	log.Printf("DEBUG: Solicitando contenido para partición ID: %s", id)

	if id == "" {
		log.Printf("DEBUG: ID de partición vacío")
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "ID de partición requerido",
		})
	}

	content, err := DiskManagement.ExploreDisk(id)
	if err != nil {
		log.Printf("DEBUG: Error en ExploreDisk: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: fmt.Sprintf("Error al explorar disco: %v", err),
		})
	}

	if len(content) == 0 {
		log.Println("DEBUG: No se encontró contenido")
		return c.JSON(nil)
	}

	return c.JSON(content[0])
}

func handlePartitionsByDisk(c *fiber.Ctx) error {
	diskName := c.Params("disk")
	partitions := DiskManagement.GetPartitionsByDisk(diskName)
	return c.JSON(partitions)
}

func handleDisks(c *fiber.Ctx) error {
	mountedPartitions := DiskManagement.GetMountedPartitions()

	var disks []DiskInfo
	for diskName, partitions := range mountedPartitions {
		disk := DiskInfo{
			Name:              diskName,
			Path:              fmt.Sprintf("%s%s.dsk", getDiskDirectory(), diskName),
			MountedPartitions: []string{},
		}

		for _, partition := range partitions {
			disk.MountedPartitions = append(disk.MountedPartitions, partition.ID)
		}

		disks = append(disks, disk)
	}

	return c.JSON(DisksResponse{Disks: disks})
}

func handleDiskPartitions(c *fiber.Ctx) error {
	diskName := c.Params("name")
	partitions := DiskManagement.GetMountedPartitions()[diskName]
	if partitions == nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
			Error: "Disco no encontrado",
		})
	}
	return c.JSON(partitions)
}

func handleTestPartition(c *fiber.Ctx) error {
	id := c.Params("id")
	log.Printf("DEBUG: Endpoint de prueba llamado con ID: %s", id)

	testContent := []fiber.Map{
		{
			"name": "root",
			"type": "folder",
			"children": []fiber.Map{
				{
					"name": "archivo1.txt",
					"type": "file",
				},
				{
					"name": "carpeta1",
					"type": "folder",
					"children": []fiber.Map{
						{
							"name": "archivo2.txt",
							"type": "file",
						},
					},
				},
			},
		},
	}

	return c.JSON(testContent)
}

func handleAllDisks(c *fiber.Ctx) error {
	diskDir := getDiskDirectory()
	files, err := ioutil.ReadDir(diskDir)
	if err != nil {
		log.Printf("Error leyendo directorio de discos %s: %v", diskDir, err)
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: "No se pudo leer la carpeta de discos",
		})
	}
	
	var disks []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".dsk") {
			disks = append(disks, strings.TrimSuffix(file.Name(), ".dsk"))
		}
	}
	
	return c.JSON(AllDisksResponse{Disks: disks})
}
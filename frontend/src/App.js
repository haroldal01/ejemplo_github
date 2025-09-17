import { useState, useEffect } from "react";
import DiskSelector from "./DiskSelector";
import PartitionSelector from "./PartitionSelector";
import PartitionViewer from "./PartitionViewer";
import LoginForm from "./LoginForm";
import "./App.css";

function App() {
    const [showLoginForm, setShowLoginForm] = useState(false);
    const [userData, setUserData] = useState(null);
    const [commands, setCommands] = useState("");
    const [output, setOutput] = useState("");
    const [isLoading, setIsLoading] = useState(false);
    const [view, setView] = useState("main"); // 'main', 'selector', 'partitions', 'disk'
    const [selectedDisk, setSelectedDisk] = useState(null);
    const [selectedPartition, setSelectedPartition] = useState(null);
    // Estados para pausa de scripts
    const [isPaused, setIsPaused] = useState(false);
    const [remainingLines, setRemainingLines] = useState([]);
    // Estado para confirmación de rmdisk
    const [confirmData, setConfirmData] = useState(null);
    // Estado para healthcheck
    const [backendStatus, setBackendStatus] = useState("checking");
    // Nuevo estado para comando individual
    //const [singleCommand, setSingleCommand] = useState("");

    // Función para verificar el estado del backend
    const checkBackendHealth = async () => {
        try {
            const response = await fetch("http://34.207.72.129:8080/health");
            if (response.ok) {
                setBackendStatus("connected");
            } else {
                setBackendStatus("error");
            }
        } catch (error) {
            setBackendStatus("error");
        }
    };

    // Verificar healthcheck al cargar la aplicación
    useEffect(() => {
        checkBackendHealth();
        // Verificar cada 30 segundos
        const interval = setInterval(checkBackendHealth, 30000);
        return () => clearInterval(interval);
    }, []);

    const handleLogin = (user) => {
        setUserData(user);
        setShowLoginForm(false);
        setView("selector");
    };

    const handleSelectDisk = (disk) => {
        setSelectedDisk(disk);
        setSelectedPartition(null);
        setView("partitions");
    };

    const handleSelectPartition = (partition) => {
        setSelectedPartition(partition);
        setView("disk");
    };

    const handleLogout = async () => {
        try {
            const res = await fetch("http://34.207.72.129:8080/logout", {
                method: "POST",
            });
            if (res.ok) {
                setUserData(null);
                setSelectedDisk(null);
                setSelectedPartition(null);
                setView("main");
                alert("Sesión cerrada correctamente");
            }
        } catch {
            alert("Error cerrando sesión");
        }
    };

    // Ejecutar comandos (primer envío)
    const executeCommands = async (scriptOverride) => {
        const scriptToRun = scriptOverride !== undefined ? scriptOverride : commands;
        if (!scriptToRun.trim()) {
            alert("Por favor, ingrese comandos para ejecutar.");
            return;
        }
        setIsLoading(true);
        setIsPaused(false);
        setRemainingLines([]);
        setConfirmData(null);
        try {
            const response = await fetch("http://34.207.72.129:8080/api/executeScript", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({ script: scriptToRun }),
            });
            const data = await response.json();
            setOutput((prev) => (prev ? prev + "\n" : "") + (data.console || ""));
            if (data.confirm) {
                setConfirmData({
                    message: data.message,
                    originalScript: scriptToRun,
                    remaining: data.remaining || [],
                });
            } else if (data.paused) {
                setIsPaused(true);
                setRemainingLines(data.remaining || []);
            } else {
                setIsPaused(false);
                setRemainingLines([]);
            }
        } catch (error) {
            setOutput("Error al ejecutar comandos: " + error.message);
        } finally {
            setIsLoading(false);
        }
    };

    // Continuar después de pausa
    const handleContinuePause = async () => {
        setIsLoading(true);
        try {
            const response = await fetch("http://34.207.72.129:8080/api/continueScript", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ remaining: remainingLines }),
            });
            const data = await response.json();
            setOutput((prev) => (prev ? prev + "\n" : "") + (data.console || ""));
            if (data.confirm) {
                setConfirmData({
                    message: data.message,
                    originalScript: remainingLines.join("\n"),
                    remaining: data.remaining || [],
                });
                setIsPaused(false);
                setRemainingLines([]);
            } else if (data.paused) {
                setIsPaused(true);
                setRemainingLines(data.remaining || []);
            } else {
                setIsPaused(false);
                setRemainingLines([]);
            }
        } catch (error) {
            setOutput("Error al continuar: " + error.message);
        } finally {
            setIsLoading(false);
        }
    };
    /*
    // Ejecutar comando individual (por ejemplo, para rmdisk)
    const executeSingleCommand = async () => {
        if (!singleCommand.trim()) return;
        setIsLoading(true);
        setConfirmData(null);
        try {
            const res = await fetch("http://localhost:8080/api/execute", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ input: singleCommand }),
            });
            const data = await res.json();
            if (data.confirm) {
                setConfirmData({
                    message: data.message,
                    original: singleCommand
                });
            } else {
                setOutput(data.console || data.message || "Sin salida");
            }
        } finally {
            setIsLoading(false);
        }
    };
    */

    // Confirmar eliminación de disco (afirmativo)
    const handleConfirmRmdisk = async () => {
        setIsLoading(true);
        // Buscar la línea de rmdisk pendiente y agregar -confirm=true
        const lines = confirmData.originalScript.split(/\r?\n/);
        let found = false;
        const newLines = lines.map(line => {
            if (!found && line.trim().toLowerCase().startsWith("rmdisk") && !line.includes("-confirm=true")) {
                found = true;
                return line.trim() + " -confirm=true";
            }
            return line;
        });
        // Ejecutar el resto del script (incluyendo la línea modificada)
        try {
            await executeCommands(newLines.join("\n"));
        } finally {
            setConfirmData(null);
            setIsLoading(false);
        }
    };

    // Cancelar eliminación de disco (negativo)
    const handleCancelRmdisk = async () => {
        setIsLoading(true);
        // Omitir la línea de rmdisk pendiente y continuar con el resto del script
        const lines = confirmData.originalScript.split(/\r?\n/);
        let found = false;
        const newLines = lines.filter(line => {
            if (!found && line.trim().toLowerCase().startsWith("rmdisk") && !line.includes("-confirm=true")) {
                found = true;
                return false; // omitir esta línea
            }
            return true;
        });
        // Ejecutar el resto del script (sin la línea de rmdisk)
        try {
            await executeCommands(newLines.join("\n"));
        } finally {
            setConfirmData(null);
            setIsLoading(false);
        }
    };

    const clearAll = () => {
        setCommands("");
        setOutput("");
        setIsPaused(false);
        setRemainingLines([]);
        setConfirmData(null);
        //setSingleCommand("");
    };

    const handleFileUpload = (event) => {
        const file = event.target.files[0];
        if (file) {
            const reader = new FileReader();
            reader.onload = (e) => {
                setCommands(e.target.result);
            };
            reader.readAsText(file);
        }
    };

    return (
        <div className="command-executor">
            {userData && (
                <button
                    onClick={handleLogout}
                    style={{
                        backgroundColor: "#c0392b",
                        color: "white",
                        border: "none",
                        orderRadius: "6px",
                        padding: "0.5rem 1rem",
                        margin: "1rem",
                        cursor: "pointer",
                        fontWeight: "bold",
                    }}
                >
                    Cerrar sesión
                </button>
            )}

            {view === "main" && (
                <>
                    <div className="header">
                        <h1>PROYECTO 2 MIA</h1>
                        <p className="subtitle">Sistema de Archivos</p>
                        
                        {/* Indicador de estado del backend */}
                        <div style={{
                            display: "flex",
                            alignItems: "center",
                            gap: "0.5rem",
                            marginBottom: "1rem",
                            padding: "0.5rem",
                            borderRadius: "6px",
                            backgroundColor: backendStatus === "connected" ? "#d4edda" : backendStatus === "error" ? "#f8d7da" : "#fff3cd",
                            color: backendStatus === "connected" ? "#155724" : backendStatus === "error" ? "#721c24" : "#856404",
                            border: `1px solid ${backendStatus === "connected" ? "#c3e6cb" : backendStatus === "error" ? "#f5c6cb" : "#ffeaa7"}`
                        }}>
                            <div style={{
                                width: "10px",
                                height: "10px",
                                borderRadius: "50%",
                                backgroundColor: backendStatus === "connected" ? "#28a745" : backendStatus === "error" ? "#dc3545" : "#ffc107",
                                animation: backendStatus === "checking" ? "pulse 1.5s infinite" : "none"
                            }}></div>
                            <span style={{ fontWeight: "bold" }}>
                                {backendStatus === "connected" ? "✅ Backend conectado" : 
                                 backendStatus === "error" ? "❌ Backend desconectado" : 
                                 "⏳ Verificando conexión..."}
                            </span>
                        </div>
                        
                        {!userData && (
                            <button
                                onClick={() => setShowLoginForm(true)}
                                style={{
                                    backgroundColor: "#3498db",
                                    color: "white",
                                    border: "none",
                                    borderRadius: "6px",
                                    padding: "0.5rem 1rem",
                                    cursor: "pointer",
                                    fontWeight: "bold",
                                }}
                            >
                                Iniciar Sesión
                            </button>
                        )}
                    </div>

                    {/* Confirmación de rmdisk */}
                    {confirmData && (
                        <div style={{margin: "1rem 0", background: "#ffe0e0", padding: "1rem", borderRadius: "8px"}}>
                            <p>{confirmData.message.replace("CONFIRM_RMDISK:", "")}</p>
                            <button onClick={handleConfirmRmdisk} disabled={isLoading} style={{background: "#c0392b", color: "white", fontWeight: "bold", border: "none", borderRadius: "6px", padding: "0.5rem 1rem"}}>
                                {isLoading ? "Eliminando..." : "Eliminar"}
                            </button>
                            <button onClick={handleCancelRmdisk} disabled={isLoading} style={{marginLeft: "1rem"}}>Cancelar</button>
                        </div>
                    )}

                    <div className="command-input-container">
                        <label htmlFor="commands-textarea">Comandos:</label>
                        <textarea
                            id="commands-textarea"
                            className="command-input"
                            rows={6}
                            placeholder="Ingrese los comandos aquí..."
                            value={commands}
                            onChange={(e) => setCommands(e.target.value)}
                            disabled={isLoading || !!confirmData}
                        ></textarea>
                    </div>

                    <div className="actions-container">
                        <div className="file-input-container">
                            <label htmlFor="file-upload" className="file-input-label">
                                Seleccionar archivo
                            </label>
                            <input id="file-upload" type="file" onChange={handleFileUpload} />
                        </div>
                        <button
                            className={`execute-button ${isLoading ? "loading" : ""}`}
                            onClick={() => executeCommands()}
                            disabled={isLoading || isPaused || !!confirmData}
                        >
                            {isLoading ? "Ejecutando..." : "Ejecutar"}
                        </button>
                        <button className="clear-button" onClick={clearAll}>
                            Limpiar
                        </button>
                        {isPaused && (
                            <button onClick={handleContinuePause} disabled={isLoading} style={{marginLeft: '1rem', backgroundColor: '#27ae60', color: 'white', border: 'none', borderRadius: '6px', padding: '0.5rem 1rem', fontWeight: 'bold', cursor: 'pointer'}}>
                                {isLoading ? "Continuando..." : "Continuar"}
                            </button>
                        )}
                    </div>

                    <div className="output-container">
                        <h2>Salida:</h2>
                        <pre className="output">{output}</pre>
                    </div>
                </>
            )}

            {view === "selector" && userData && (
                <div>
                    <button
                        onClick={() => setView("main")}
                        style={{
                            backgroundColor: "#888",
                            color: "white",
                            border: "none",
                            borderRadius: "6px",
                            padding: "0.5rem 1rem",
                            margin: "1rem",
                            cursor: "pointer",
                            fontWeight: "bold",
                        }}
                    >
                        Volver
                    </button>
                    <DiskSelector onSelectDisk={handleSelectDisk} />
                </div>
            )}

            {view === "partitions" && userData && selectedDisk && (
                <div>
                    <button
                        onClick={() => setView("selector")}
                        style={{
                            backgroundColor: "#888",
                            color: "white",
                            border: "none",
                            borderRadius: "6px",
                            padding: "0.5rem 1rem",
                            margin: "1rem",
                            cursor: "pointer",
                            fontWeight: "bold",
                        }}
                    >
                        Volver a discos
                    </button>
                    <PartitionSelector
                        disk={selectedDisk}
                        onSelectPartition={handleSelectPartition}
                        onBack={() => setView("selector")}
                    />
                </div>
            )}

            {view === "disk" && userData && selectedDisk && selectedPartition && (
                <div>
                    <button
                        onClick={() => setView("partitions")}
                        style={{
                            backgroundColor: "#888",
                            color: "white",
                            border: "none",
                            borderRadius: "6px",
                            padding: "0.5rem 1rem",
                            margin: "1rem",
                            cursor: "pointer",
                            fontWeight: "bold",
                        }}
                    >
                        Volver a particiones
                    </button>
                    <PartitionViewer 
                        partition={selectedPartition} 
                        onBack={() => setView("partitions")} 
                    />
                </div>
            )}

            {showLoginForm && (
                <div className="login-overlay" style={{
                    position: "fixed",
                    top: 0,
                    left: 0,
                    right: 0,
                    bottom: 0,
                    backgroundColor: "rgba(0, 0, 0, 0.5)",
                    display: "flex",
                    justifyContent: "center",
                    alignItems: "center",
                    zIndex: 1000
                }}>
                    <div style={{ position: 'relative' }}>
                        <button
                            onClick={() => setShowLoginForm(false)}
                            style={{
                                position: 'absolute',
                                top: '-2.5rem',
                                right: 0,
                                background: '#c0392b',
                                color: 'white',
                                border: 'none',
                                borderRadius: '6px',
                                padding: '0.5rem 1rem',
                                fontWeight: 'bold',
                                cursor: 'pointer',
                                zIndex: 1001
                            }}
                        >
                            Cerrar
                        </button>
                        <LoginForm onLogin={handleLogin} />
                    </div>
                </div>
            )}
        </div>
    );
}

export default App;
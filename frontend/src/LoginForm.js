import React, { useState } from "react";

function LoginForm({ onLogin }) {
  const [partitionId, setPartitionId] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [rememberUser, setRememberUser] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
  
    if (!partitionId || !username || !password) {
      alert("Todos los campos son obligatorios.");
      return;
    }
  
    try {
              const response = await fetch("http://34.207.72.129:8080/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ username, password, partition_id: partitionId }),
      });
  
      if (response.ok) {
        alert("Inicio de sesión exitoso");
        onLogin({ username, partitionId, rememberUser });
      } else {
        alert("Usuario o contraseña incorrectos o partición no montada");
      }
    } catch (error) {
      alert("Error al comunicarse con el backend.");
    }
  };
  

  return (
    <div style={styles.container}>
      <h2 style={styles.title}>Iniciar Sesión</h2>
      <form onSubmit={handleSubmit}>
        <label style={styles.label}>ID Partición:</label>
        <input
          type="text"
          placeholder="Ej: A100"
          value={partitionId}
          onChange={(e) => setPartitionId(e.target.value)}
          style={styles.input}
        />

        <label style={styles.label}>Usuario:</label>
        <input
          type="text"
          placeholder=""
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          style={styles.input}
        />

        <label style={styles.label}>Contraseña:</label>
        <input
          type="password"
          placeholder="******"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          style={styles.input}
        />

        <div style={styles.checkboxContainer}>
          <input
            type="checkbox"
            id="recordar"
            checked={rememberUser}
            onChange={(e) => setRememberUser(e.target.checked)}
          />
          <label htmlFor="recordar" style={styles.checkboxLabel}>
            Recordar usuario
          </label>
        </div>

        <button type="submit" style={styles.button}>
          Submit
        </button>
      </form>
    </div>
  );
}

// 🎨 Estilos
const styles = {
  container: {
    backgroundColor: "#2e4a4e", // Fondo solicitado
    padding: "2rem",
    borderRadius: "12px",
    width: "320px",
    boxShadow: "0 4px 12px rgba(247, 247, 247, 0.3)",
    textAlign: "left",
    fontFamily: "Segoe UI, sans-serif",
    color: "#eafafa",
  },
  title: {
    textAlign: "center",
    marginBottom: "1.5rem",
    color: "#c2f3f4",
  },
  label: {
    fontWeight: 600,
    fontSize: "0.95rem",
    marginBottom: "0.25rem",
    display: "block",
  },
  input: {
    width: "100%",
    padding: "0.5rem",
    marginBottom: "1rem",
    borderRadius: "6px",
    border: "1px solid #aacfcf",
    backgroundColor: "#eaf7f9",
    fontSize: "1rem",
    color: "#1d3c42",
  },
  checkboxContainer: {
    display: "flex",
    alignItems: "center",
    marginBottom: "1.2rem",
  },
  checkboxLabel: {
    marginLeft: "0.5rem",
    fontSize: "0.9rem",
  },
  button: {
    width: "100%",
    padding: "0.6rem",
    backgroundColor: "#4da6b3",
    color: "white",
    fontWeight: "bold",
    border: "none",
    borderRadius: "6px",
    fontSize: "1rem",
    cursor: "pointer",
    transition: "background-color 0.2s ease-in-out",
  },
};

export default LoginForm;
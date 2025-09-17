import React, { useEffect, useState } from "react";

function DiskSelector({ onSelectDisk }) {
  const [disks, setDisks] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
            fetch("http://34.207.72.129:8080/api/disks")
      .then((res) => res.json())
      .then((data) => {
        setDisks(data.disks || []);
        setLoading(false);
      })
      .catch((err) => {
        console.error("Error al cargar discos:", err);
        setError("Error al cargar discos");
        setLoading(false);
      });
  }, []);

  const handleSelect = (disk) => {
    if (onSelectDisk) {
      onSelectDisk(disk);
    }
  };

  if (loading) return <div style={styles.container}>Cargando discos...</div>;
  if (error) return <div style={styles.container}><p style={{ color: "red" }}>{error}</p></div>;

  return (
    <div style={styles.container}>
      <h2 style={styles.title}>Visualizador del Sistema de Archivos</h2>
      <p style={styles.subtitle}>Seleccione el disco que desea visualizar:</p>
      <div style={styles.grid}>
        {disks.map((disk) => (
          <div
            key={disk.name}
            style={cardStyle}
            onClick={() => handleSelect(disk)}
          >
            <h3>{disk.name}</h3>
            <p><strong>Ruta:</strong> {disk.path}</p>
            <p><strong>Particiones:</strong> {disk.mounted_partitions?.join(", ") || "Ninguna"}</p>
          </div>
        ))}
      </div>
    </div>
  );
}

const styles = {
  container: {
    textAlign: "center",
    padding: "2rem",
    fontFamily: "Segoe UI, sans-serif",
  },
  title: {
    fontSize: "1.6rem",
    marginBottom: "0.5rem",
  },
  subtitle: {
    marginBottom: "1.5rem",
    color: "#555",
  },
  grid: {
    display: "flex",
    justifyContent: "center",
    gap: "1.5rem",
    flexWrap: "wrap",
  },
  card: {
    backgroundColor: "#eafafa",
    borderRadius: "10px",
    padding: "1.2rem",
    cursor: "pointer",
    width: "250px",
    boxShadow: "0 4px 12px rgba(0,0,0,0.1)",
    transition: "transform 0.2s, box-shadow 0.2s",
    border: "1px solid #ddd",
  },
};

const cardStyle = {
  ...styles.card,
  ":hover": {
    transform: "translateY(-5px)",
    boxShadow: "0 8px 20px rgba(0,0,0,0.15)",
  }
};

export default DiskSelector;
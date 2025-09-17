import React, { useEffect, useState } from "react";

function DiskViewer({ disk, onBack }) {
  const [partitions, setPartitions] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (disk) {
      setLoading(true);
              fetch(`http://34.207.72.129:8080/api/partitions/${disk.name}`)
        .then(res => res.json())
        .then(data => {
          setPartitions(data);
          setLoading(false);
        })
        .catch(() => {
          setPartitions([]);
          setLoading(false);
        });
    }
  }, [disk]);

  if (!disk) return null;

  return (
    <div style={styles.container}>
      <div style={styles.infoBox}>
        <p><strong>Ruta:</strong> {disk.path}</p>
      </div>
      <div style={styles.treeBox}>
        <h3>Particiones montadas:</h3>
        {loading ? (
          <p>Cargando particiones...</p>
        ) : partitions.length > 0 ? (
          <ul>
            {partitions.map((part, index) => (
              <li key={index}>
                <strong>{part.name}</strong> (ID: {part.id}, Tipo: {part.type}, Estado: {part.status}, Inicio: {part.start}, Tamaño: {part.size})
              </li>
            ))}
          </ul>
        ) : (
          <p>No hay particiones montadas</p>
        )}
      </div>
    </div>
  );
}

const styles = {
  container: {
    padding: "2rem",
    fontFamily: "Segoe UI, sans-serif",
  },
  title: {
    fontSize: "1.8rem",
    marginBottom: "1rem",
  },
  infoBox: {
    backgroundColor: "#f2f2f2",
    padding: "1rem",
    borderRadius: "8px",
    marginBottom: "1rem",
    cursor: "pointer",
    transition: "background 0.2s",
  },
  treeBox: {
    marginTop: "1rem",
    padding: "1rem",
    backgroundColor: "#ffffff",
    border: "1px dashed #ccc",
    borderRadius: "6px",
  },
  backButton: {
    marginBottom: "1rem",
    background: "#888",
    color: "#fff",
    border: "none",
    borderRadius: "6px",
    padding: "0.5rem 1rem",
    cursor: "pointer",
    fontWeight: "bold",
  },
};

export default DiskViewer;

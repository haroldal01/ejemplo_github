import React, { useEffect, useState } from "react";

function PartitionSelector({ disk, onSelectPartition, onBack }) {
  const [partitions, setPartitions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (disk) {
      setLoading(true);
              fetch(`http://34.207.72.129:8080/api/partitions/${disk.name}`)
        .then(res => res.json())
        .then(data => {
          setPartitions(data);
          setLoading(false);
        })
        .catch(err => {
          console.error("Error al cargar particiones:", err);
          setError("Error al cargar particiones");
          setLoading(false);
        });
    }
  }, [disk]);

  const handleSelect = (partition) => {
    if (onSelectPartition) {
      onSelectPartition(partition);
    }
  };

  if (!disk) return null;
  if (loading) return <div style={styles.container}>Cargando particiones...</div>;
  if (error) return <div style={styles.container}><p style={{ color: "red" }}>{error}</p></div>;

  return (
    <div style={styles.container}>
      <h2 style={styles.title}>Particiones de {disk.name}</h2>
      <p style={styles.subtitle}>Seleccione la partición que desea explorar:</p>
      <div style={styles.grid}>
        {partitions.length > 0 ? (
          partitions.map((partition, index) => (
            <div
              key={index}
              style={cardStyle}
              onClick={() => handleSelect(partition)}
            >
              <h3>{partition.name}</h3>
              <p><strong>ID:</strong> {partition.id}</p>
              <p><strong>Tipo:</strong> {partition.type}</p>
              <p><strong>Estado:</strong> {partition.status}</p>
              <p><strong>Inicio:</strong> {partition.start}</p>
              <p><strong>Tamaño:</strong> {partition.size} bytes</p>
            </div>
          ))
        ) : (
          <p style={{ color: "red" }}>No hay particiones montadas en este disco.</p>
        )}
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

export default PartitionSelector;
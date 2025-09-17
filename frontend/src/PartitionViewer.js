import React, { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { Treebeard } from "react-treebeard";

function PartitionViewer() {
  const { id } = useParams();
  const [treeData, setTreeData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [cursor, setCursor] = useState(null);

  useEffect(() => {
    const fetchTree = async () => {
      try {
        const res = await fetch(`http://34.207.72.129:8080/api/partition-content/${id}`);
        const data = await res.json();
        setTreeData(data[0]); // Suponiendo que el backend devuelve un array con un nodo raíz
      } catch (err) {
        console.error("Error cargando árbol de partición:", err);
      } finally {
        setLoading(false);
      }
    };

    fetchTree();
  }, [id]);

  const onToggle = (node, toggled) => {
    if (cursor) {
      cursor.active = false;
    }
    node.active = true;
    if (node.children) {
      node.toggled = toggled;
    }
    setCursor(node);
    setTreeData({ ...treeData });
  };

  return (
    <div style={{ padding: "1rem", backgroundColor: "#fff" }}>
      <h3>Contenido de la partición {id}</h3>
      {loading ? (
        <p>Cargando contenido de la partición...</p>
      ) : treeData ? (
        <Treebeard data={treeData} onToggle={onToggle} />
      ) : (
        <p>No se pudo cargar el contenido.</p>
      )}
    </div>
  );
}

export default PartitionViewer;


import React, { useEffect, useState } from "react";
import { Treebeard } from "react-treebeard";

function DiskTree({ partitionId }) {
  const [data, setData] = useState(null);

  useEffect(() => {
            fetch(`http://34.207.72.129:8080/api/disk-tree/${partitionId}`)
      .then(res => res.json())
      .then(setData)
      .catch(() => alert("Error cargando árbol del disco"));
  }, [partitionId]);

  return (
    <div>
      {data && data.tree && <Treebeard data={data.tree} />}

    </div>
  );
}

export default DiskTree;

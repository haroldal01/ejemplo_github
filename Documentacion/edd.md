El sistema de archivos implementado en el proyecto MIA_P2 utiliza estructuras de datos fundamentales basadas en un sistema de archivos similar a ext2/ext3, incluyendo el **MBR (Master Boot Record)**, **inodos**, y **bloques**. Estas estructuras permiten gestionar discos, particiones, y archivos dentro de un archivo binario que simula un disco físico. A continuación, se describen cada una de estas estructuras, su función en el sistema de archivos, y cómo se organizan y gestionan dentro del archivo binario.

## 1. Master Boot Record (MBR)

### Descripción y Función
El **MBR** es la primera estructura almacenada en el archivo binario que representa un disco. Su función principal es almacenar metadatos sobre el disco y la tabla de particiones, permitiendo la gestión de hasta cuatro particiones primarias o extendidas. El MBR actúa como el punto de entrada para identificar las particiones disponibles y sus ubicaciones en el disco.

### Estructura
La estructura del MBR en el sistema está definida en el archivo `DiskManagement.go` como `Structs.MBR` y contiene los siguientes campos:

- **mbr_tamano (int32)**: Tamaño total del disco en bytes.
- **mbr_fecha_creacion ([20]byte)**: Fecha y hora de creación del disco, almacenada como una cadena de texto.
- **mbr_dsk_signature (int32)**: Identificador único del disco, generado aleatoriamente.
- **dsk_fit (byte)**: Tipo de ajuste para las particiones (BF: Best Fit, FF: First Fit, WF: Worst Fit).
- **mbr_partition_1, mbr_partition_2, mbr_partition_3, mbr_partition_4 (Structs.Partition)**: Cuatro entradas para particiones primarias o extendidas, cada una con los siguientes campos:
  - **part_status ([1]byte)**: Estado de la partición (`0` inactiva, `1` activa).
  - **part_type ([1]byte)**: Tipo de partición (`P` para primaria, `E` para extendida).
  - **part_fit ([1]byte)**: Ajuste de la partición (`B` para Best Fit, `F` para First Fit, `W` para Worst Fit).
  - **part_start (int32)**: Byte inicial de la partición en el disco.
  - **part_size (int32)**: Tamaño de la partición en bytes.
  - **part_name ([16]byte)**: Nombre de la partición (hasta 16 caracteres).
  - **part_id ([16]byte)**: Identificador único de la partición (generado al montar).

### Organización en el Archivo Binario
- **Ubicación**: El MBR se escribe al inicio del archivo binario (offset 0).
- **Tamaño**: El tamaño del MBR es fijo y se calcula como la suma de los tamaños de sus campos:
  - `mbr_tamano`: 4 bytes
  - `mbr_fecha_creacion`: 20 bytes
  - `mbr_dsk_signature`: 4 bytes
  - `dsk_fit`: 1 byte
  - `mbr_partition_1` a `mbr_partition_4`: 4 * (1 + 1 + 1 + 4 + 4 + 16 + 16) = 4 * 43 = 172 bytes
  - **Total**: 4 + 20 + 4 + 1 + 172 = 201 bytes
- **Gestión**: Al crear un disco con `Mkdisk`, el MBR se inicializa con los valores proporcionados (tamaño, ajuste, etc.) y se escribe en el archivo binario. Las particiones se actualizan con `Fdisk` para modificar `part_status`, `part_type`, `part_fit`, `part_start`, `part_size`, y `part_name`. El `part_id` se asigna al montar la partición con `Mount`.

### Gestión
- **Creación**: La función `Mkdisk` en `DiskManagement.go` crea el archivo binario (`.dsk`) y escribe el MBR con los valores iniciales. Las particiones se inicializan con `part_status` en `0` (inactivas).
- **Modificación**: La función `Fdisk` actualiza las entradas de particiones en el MBR para crear, eliminar o redimensionar particiones. Verifica restricciones como no exceder el tamaño del disco o solaparse con otras particiones.
- **Montaje**: La función `Mount` genera un `part_id` único y lo almacena en la partición correspondiente, manteniendo una lista global de particiones montadas en `stores.ListMountedPartitions`.
- **Eliminación**: La función `Rmdisk` elimina el archivo binario, borrando el MBR y todas las particiones asociadas.
- **Reportes**: La función `ReportMBR` genera un reporte gráfico del MBR usando Graphviz, mostrando el tamaño del disco y las particiones con sus atributos.

## 2. Inodos

### Descripción y Función
Los **inodos** son estructuras que almacenan metadatos sobre archivos y directorios en una partición formateada con el sistema de archivos (`2FS` o `3FS`). Cada inodo representa un archivo o directorio y contiene información sobre su propietario, permisos, tamaño, fechas, y punteros a los bloques de datos que almacenan su contenido.

### Estructura
La estructura del inodo está definida en `UserManager.go` como `Structs.Inode` y contiene los siguientes campos:

- **i_uid (int32)**: Identificador del usuario propietario.
- **i_gid (int32)**: Identificador del grupo propietario.
- **i_size (int32)**: Tamaño del archivo en bytes.
- **i_atime ([20]byte)**: Fecha de último acceso.
- **i_ctime ([20]byte)**: Fecha de creación.
- **i_mtime ([20]byte)**: Fecha de última modificación.
- **i_block ([15]int32)**: Arreglo de 15 punteros a bloques:
  - 12 punteros directos a bloques de datos.
  - 1 puntero indirecto simple.
  - 1 puntero indirecto doble.
  - 1 puntero indirecto triple.
- **i_type ([1]byte)**: Tipo de inodo (`0` para archivo, `1` para directorio).
- **i_perm (int32)**: Permisos en formato numérico (e.g., 664).
- **i_nlink (int32)**: Número de enlaces al inodo.

### Organización en el Archivo Binario
- **Ubicación**: Los inodos se almacenan en una tabla de inodos dentro de la partición, cuyo inicio está definido en el superbloque (`s_inode_start`). Cada inodo ocupa un tamaño fijo, calculado como:
  - `i_uid`, `i_gid`, `i_size`, `i_perm`, `i_nlink`: 5 * 4 = 20 bytes
  - `i_atime`, `i_ctime`, `i_mtime`: 3 * 20 = 60 bytes
  - `i_block`: 15 * 4 = 60 bytes
  - `i_type`: 1 byte
  - **Total**: 20 + 60 + 60 + 1 = 141 bytes
- **Tabla de Inodos**: La tabla de inodos es un arreglo contiguo de inodos, con un bitmap de inodos (`bm_inode`) que indica cuáles están en uso (`1`) o libres (`0`).
- **Gestión**: 
  - **Creación**: La función `Mkfs` inicializa la tabla de inodos y el bitmap de inodos. El primer inodo (inodo 0) se reserva para el directorio raíz (`/`) o el archivo `users.txt` en el caso del sistema de usuarios.
  - **Modificación**: Funciones como `Mkdir` y `Mkfile` crean nuevos inodos, actualizan el bitmap de inodos, y asignan bloques de datos según sea necesario.
  - **Acceso**: La función `GetInode` lee un inodo específico desde la tabla de inodos, utilizando el offset calculado como `s_inode_start + (inode_number * 141)`.
  - **Reportes**: Los reportes `InodeReport` y `BmInodeReport` generan representaciones gráficas y textuales de la tabla de inodos y el bitmap de inodos, respectivamente.

### Gestión
- **Inicialización**: Durante el formateo (`Mkfs`), se calcula el número de inodos según el tamaño de la partición y se inicializa el bitmap de inodos con ceros, excepto para el inodo 0 (directorio raíz o `users.txt`).
- **Asignación**: Al crear un archivo o directorio, se busca el primer inodo libre en el bitmap (`0`), se marca como ocupado (`1`), y se escribe el inodo con los metadatos correspondientes.
- **Punteros a Bloques**: Los inodos usan un esquema de indexación con 12 punteros directos y 3 indirectos. Para archivos grandes, los punteros indirectos apuntan a bloques de punteros, que a su vez apuntan a bloques de datos.
- **Eliminación**: No se implementa explícitamente la eliminación de inodos, pero al reformatear una partición (`Mkfs` con `type=FULL`), la tabla de inodos se reinicia.
- **Reportes**: Los inodos se visualizan en el reporte `inode`, que muestra sus atributos, y en `bm_inode`, que muestra el bitmap como una secuencia de `0` y `1`.

## 3. Bloques

### Descripción y Función
Los **bloques** son las unidades de almacenamiento de datos en el sistema de archivos. Hay dos tipos principales de bloques:
- **Bloques de Carpetas (Folder Blocks)**: Almacenan entradas de directorios, que asocian nombres de archivos o subdirectorios con números de inodo.
- **Bloques de Archivos (File Blocks)**: Almacenan el contenido de los archivos.

### Estructura
- **Bloque de Carpeta (`Structs.FolderBlock`)**:
  - **b_content ([4]Structs.Content)**: Cuatro entradas de contenido, cada una con:
    - **b_name ([12]byte)**: Nombre del archivo o directorio (hasta 12 caracteres).
    - **b_inodo (int32)**: Número de inodo asociado.
  - **Tamaño**: 4 * (12 + 4) = 64 bytes.
- **Bloque de Archivo (`Structs.FileBlock`)**:
  - **b_content ([64]byte)**: Contenido del archivo (hasta 64 bytes por bloque).
  - **Tamaño**: 64 bytes.
- **Bloque de Punteros (`Structs.PointerBlock`)**:
  - **b_pointers ([16]int32)**: 16 punteros a otros bloques (usado para indexación indirecta).
  - **Tamaño**: 16 * 4 = 64 bytes.

### Organización en el Archivo Binario
- **Ubicación**: Los bloques se almacenan en una tabla de bloques dentro de la partición, cuyo inicio está definido en el superbloque (`s_block_start`). Cada bloque, independientemente de su tipo, ocupa 64 bytes.
- **Tabla de Bloques**: Es un arreglo contiguo de bloques, con un bitmap de bloques (`bm_block`) que indica cuáles están en uso (`1`) o libres (`0`).
- **Gestión**:
  - **Creación**: `Mkfs` inicializa la tabla de bloques y el bitmap de bloques. El primer bloque (bloque 0) se usa para el directorio raíz o el contenido inicial de `users.txt`.
  - **Modificación**: `Mkdir` crea bloques de carpeta para almacenar entradas de directorio. `Mkfile` crea bloques de archivo para almacenar contenido, asignando bloques adicionales si el archivo excede los 64 bytes por bloque.
  - **Acceso**: La función `GetBlock` lee un bloque específico desde la tabla de bloques, utilizando el offset `s_block_start + (block_number * 64)`.
  - **Reportes**: Los reportes `BlockReport` y `BmBlockReport` generan representaciones gráficas y textuales de la tabla de bloques y el bitmap de bloques, respectivamente.

### Gestión
- **Inicialización**: Durante el formateo, se calcula el número de bloques según el tamaño de la partición. El bitmap de bloques se inicializa con ceros, excepto para los bloques usados por el directorio raíz o `users.txt`.
- **Asignación**: Al crear un archivo o directorio, se busca el primer bloque libre en el bitmap (`0`), se marca como ocupado (`1`), y se escribe el bloque con el contenido correspondiente.
- **Indexación**: Los inodos apuntan a bloques de datos. Para archivos grandes, se usan bloques de punteros para indexación indirecta (simple, doble, o triple).
- **Eliminación**: Similar a los inodos, la eliminación ocurre al reformatear la partición.
- **Reportes**: Los bloques se visualizan en el reporte `block`, que muestra su tipo y contenido, y en `bm_block`, que muestra el bitmap como una secuencia de `0` y `1`.

## 4. Superbloque

### Descripción y Función
El **superbloque** es una estructura que almacena metadatos sobre el sistema de archivos de una partición. Sirve como el mapa que describe cómo están organizados los inodos y bloques, y contiene información sobre el espacio libre, el número de inodos y bloques, y las fechas de montaje.

### Estructura
La estructura del superbloque está definida en `UserManager.go` como `Structs.Superblock` y contiene los siguientes campos:

- **s_filesystem_type (int32)**: Tipo de sistema de archivos (`2` para 2FS, `3` para 3FS).
- **s_inodes_count (int32)**: Número total de inodos.
- **s_blocks_count (int32)**: Número total de bloques.
- **s_free_inodes_count (int32)**: Número de inodos libres.
- **s_free_blocks_count (int32)**: Número de bloques libres.
- **s_mtime ([20]byte)**: Fecha de último montaje.
- **s_mnt_count (int32)**: Número de veces que se ha montado la partición.
- **s_magic (int32)**: Valor mágico para identificar el sistema de archivos (e.g., `0xEF53`).
- **s_inode_size (int32)**: Tamaño de cada inodo (141 bytes).
- **s_block_size (int32)**: Tamaño de cada bloque (64 bytes).
- **s_first_ino (int32)**: Primer inodo libre.
- **s_first_blo (int32)**: Primer bloque libre.
- **s_bm_inode_start (int32)**: Byte inicial del bitmap de inodos.
- **s_bm_block_start (int32)**: Byte inicial del bitmap de bloques.
- **s_inode_start (int32)**: Byte inicial de la tabla de inodos.
- **s_block_start (int32)**: Byte inicial de la tabla de bloques.

### Organización en el Archivo Binario
- **Ubicación**: El superbloque se escribe al inicio de la partición, inmediatamente después del MBR o la EBR (para particiones extendidas).
- **Tamaño**: Calculado como:
  - Campos `int32`: 11 * 4 = 44 bytes
  - `s_mtime`: 20 bytes
  - **Total**: 44 + 20 = 64 bytes
- **Gestión**:
  - **Creación**: `Mkfs` inicializa el superbloque con el número de inodos y bloques calculados según el tamaño de la partición, y define los offsets para los bitmaps y tablas.
  - **Actualización**: Cada vez que se crea o elimina un archivo/directorio, se actualizan `s_free_inodes_count`, `s_free_blocks_count`, `s_first_ino`, y `s_first_blo`.
  - **Acceso**: La función `GetSuperblock` lee el superbloque desde el inicio de la partición.
  - **Reportes**: El reporte `SuperBlockReport` genera una representación gráfica del superbloque con Graphviz.

## 5. Organización General en el Archivo Binario

El archivo binario (`.dsk`) que representa un disco está organizado de la siguiente manera:

1. **MBR (offset 0, 201 bytes)**: Contiene los metadatos del disco y la tabla de particiones.
2. **Particiones**:
   - **Primarias**: Comienzan en el byte especificado por `part_start`. Si están formateadas, contienen:
     - **Superbloque**: Al inicio de la partición (64 bytes).
     - **Bitmap de Inodos**: A partir de `s_bm_inode_start`.
     - **Bitmap de Bloques**: A partir de `s_bm_block_start`.
     - **Tabla de Inodos**: A partir de `s_inode_start`.
     - **Tabla de Bloques**: A partir de `s_block_start`.
   - **Extendidas**: Contienen una lista de **EBR (Extended Boot Record)**, cada una describiendo una partición lógica con un superbloque y las mismas estructuras que una partición primaria.
3. **Espacio Libre**: Los bytes no asignados a particiones o estructuras están llenos de ceros.

### Ejemplo de Organización
Para un disco de 10 MB con una partición primaria formateada:
- **Offset 0**: MBR (201 bytes).
- **Offset 201**: Inicio de la partición primaria (definido por `part_start`).
  - **Offset 201**: Superbloque (64 bytes).
  - **Offset 265**: Bitmap de inodos (e.g., 1000 bytes para 1000 inodos).
  - **Offset 1265**: Bitmap de bloques (e.g., 2000 bytes para 2000 bloques).
  - **Offset 3265**: Tabla de inodos (e.g., 1000 * 141 = 141,000 bytes).
  - **Offset 144265**: Tabla de bloques (e.g., 2000 * 64 = 128,000 bytes).
  - Resto del espacio: Bloques de datos o espacio libre.

## 6. Gestión de Estructuras

- **Creación de Disco (`Mkdisk`)**: Crea el archivo binario y escribe el MBR con particiones vacías.
- **Gestión de Particiones (`Fdisk`, `Mount`, `Unmount`)**: Actualiza el MBR y la lista de particiones montadas en memoria (`stores.ListMountedPartitions`).
- **Formateo (`Mkfs`)**: Inicializa el superbloque, bitmaps, y tablas de inodos y bloques. Crea el directorio raíz (`/`) o el archivo `users.txt` para sistemas de usuarios.
- **Gestión de Archivos y Directorios (`Mkdir`, `Mkfile`, `Cat`)**: Crea y actualiza inodos y bloques, manteniendo los bitmaps y el superbloque actualizados.
- **Reportes**: Generan representaciones gráficas (PNG via Graphviz) o textuales de las estructuras:
  - `mbr`: Muestra el MBR y las particiones.
  - `disk`: Muestra la distribución de particiones en el disco.
  - `inode`: Muestra la tabla de inodos.
  - `block`: Muestra la tabla de bloques.
  - `bm_inode` y `bm_block`: Muestran los bitmaps como texto.
  - `superblock`: Muestra los metadatos del superbloque.
  - `ls` y `file`: Muestran listados de directorios o contenido de archivos.

## 7. Consideraciones

- **Tamaño Fijo**: Todas las estructuras (MBR, superbloque, inodos, bloques) tienen tamaños fijos, lo que facilita el acceso directo mediante offsets.
- **Bitmaps**: Los bitmaps de inodos y bloques optimizan la asignación de recursos, marcando rápidamente qué inodos y bloques están libres.
- **Indexación Indirecta**: Los inodos soportan archivos grandes mediante punteros indirectos, aunque el sistema no implementa journaling (ignorando `3FS` como se especificó).
- **Persistencia**: Todas las estructuras se almacenan en el archivo binario, asegurando que los datos persistan entre sesiones.
- **Limitaciones**: El sistema soporta hasta 4 particiones por disco (límite del MBR) y no implementa eliminación directa de archivos/directorios (solo reformateo).


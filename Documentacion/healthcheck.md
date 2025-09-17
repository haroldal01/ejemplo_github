# Healthcheck del Backend

## Descripción
Se ha implementado un sistema de healthcheck para verificar el estado de conexión entre el frontend y el backend.

## Endpoints

### Backend Healthcheck
- **URL**: `GET /health`
- **Descripción**: Verifica si el backend está funcionando correctamente
- **Respuesta exitosa**:
```json
{
  "status": "ok",
  "message": "Backend funcionando correctamente",
  "timestamp": "1703123456"
}
```

## Frontend Healthcheck

### Indicador Visual
El frontend muestra un indicador visual en el header que cambia según el estado de conexión:

- **🟢 Conectado**: Backend responde correctamente
- **🔴 Desconectado**: No se puede conectar al backend
- **🟡 Verificando**: Realizando verificación de conexión

### Funcionalidad
- **Verificación automática**: Se ejecuta al cargar la aplicación
- **Verificación periódica**: Se repite cada 30 segundos
- **Indicador en tiempo real**: Muestra el estado actual de la conexión

### Funcionalidad
- **Verificación automática**: Se ejecuta al cargar la aplicación
- **Verificación periódica**: Se repite cada 30 segundos
- **Indicador en tiempo real**: Muestra el estado actual de la conexión

### Estados del Indicador
1. **"checking"**: Verificando conexión (animación de pulso)
2. **"connected"**: Backend conectado (verde)
3. **"error"**: Backend desconectado (rojo)

## Uso

### Verificar manualmente desde el navegador
```
http://34.207.72.129:8080/health
```

### Verificar desde curl
```bash
curl http://34.207.72.129:8080/health
```

### Verificar desde JavaScript
```javascript
fetch('http://34.207.72.129:8080/health')
  .then(response => response.json())
  .then(data => console.log(data));
```

## Configuración

### Puerto
El healthcheck utiliza el mismo puerto que el backend (8080).

### IP
La IP configurada es: `34.207.72.129`

### Intervalo de verificación
El frontend verifica la conexión cada 30 segundos automáticamente.

## Troubleshooting

### Si el indicador muestra "Desconectado":
1. Verificar que el backend esté ejecutándose
2. Verificar que el puerto 8080 esté abierto en el grupo de seguridad de AWS
3. Verificar que la IP sea correcta
4. Verificar la conectividad de red

### Si el indicador muestra "Verificando" por mucho tiempo:
1. Verificar la latencia de red
2. Verificar si hay problemas de DNS
3. Verificar si el backend está sobrecargado 
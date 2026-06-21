---
name: api-client-testing
description: >-
  Provides guidelines, code examples, and best practices for creating unit and
  integration tests for third-party APIs (Google Calendar, Gemini, OAuth) in Go.
  Includes mocking HTTP requests and managing testing environments.
---

# API Client Testing Guidelines in Go

Esta habilidad define cómo crear pruebas automatizadas robustas para integraciones con APIs externas en Event Hub, asegurando que las suites de pruebas se ejecuten de forma rápida, aislada y sin necesidad de realizar llamadas de red reales.

## Principios y Buenas Prácticas

### 1. Aislamiento del Entorno (Variables de Entorno)
Para evitar que las credenciales reales afecten o se filtren durante las pruebas, siempre aísla el entorno configurando valores falsos en `t.Run` o al inicio del test, y restáuralos al finalizar mediante funciones diferidas (`defer`).

#### Patrón Recomendado:
```go
func TestMiServicio(t *testing.T) {
    // Configura credenciales ficticias
    os.Setenv("EXTERNAL_API_KEY", "test-key-123")
    defer func() {
        os.Unsetenv("EXTERNAL_API_KEY")
    }()

    // Ejecución de pruebas...
}
```

### 2. Simulación de Servidores HTTP (`httptest.NewServer`)
En lugar de consumir APIs de producción (como Google Calendar o Gemini), inicia un servidor HTTP local simulado mediante la biblioteca estándar `net/http/httptest`.

#### Ejemplo de Mock Server:
```go
func TestGoogleCalendarService_Insert(t *testing.T) {
    // 1. Iniciar un servidor de pruebas local
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Validar el método y headers
        if r.Method != http.MethodPost {
            t.Errorf("se esperaba POST, se obtuvo %s", r.Method)
        }
        
        // Simular respuesta JSON exitosa
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"id": "external-event-id-999", "status": "confirmed"}`))
    }))
    defer server.Close()

    // 2. Configurar el cliente de Go para usar el URL de este servidor mock
    client := NewCalendarServiceClient(server.URL, "fake-token")
    eventID, err := client.InsertEvent("Título Evento", time.Now())
    
    // 3. Validar resultados
    if err != nil {
        t.Fatalf("se esperaba éxito, se obtuvo error: %v", err)
    }
    if eventID != "external-event-id-999" {
        t.Errorf("se esperaba id external-event-id-999, se obtuvo %s", eventID)
    }
}
```

### 3. Testeo de Casos de Falla
Garantiza la resiliencia del backend probando explícitamente cómo responde el sistema ante fallos de APIs externas:
* Retornos de estado `500 Internal Server Error` o `403 Forbidden` por parte del proveedor.
* Respuestas de payload JSON corrupto o malformado.
* Retardos de red simulados para comprobar el correcto funcionamiento de los Timeouts de contexto de Go (`context.WithTimeout`).

#### Ejemplo de Test de Excepción:
```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusInternalServerError) // Simula caída de Google
}))
defer server.Close()
```

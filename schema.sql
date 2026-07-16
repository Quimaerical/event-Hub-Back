-- ==========================================
-- 0. LIMPIEZA INICIAL Y EXTENSIONES
-- ==========================================
DROP VIEW IF EXISTS v_eventos_programados;
DROP TABLE IF EXISTS auditoria_log CASCADE;
DROP TABLE IF EXISTS recordatorios CASCADE;
DROP TABLE IF EXISTS pagos CASCADE;
DROP TABLE IF EXISTS reservas CASCADE;
DROP TABLE IF EXISTS evento_categorias CASCADE;
DROP TABLE IF EXISTS eventos CASCADE;
DROP TABLE IF EXISTS espacios CASCADE;
DROP TABLE IF EXISTS categorias CASCADE;
DROP TABLE IF EXISTS usuarios CASCADE;
DROP TABLE IF EXISTS roles CASCADE;

DROP TYPE IF EXISTS estado_evento CASCADE;
DROP TYPE IF EXISTS estado_reserva CASCADE;
DROP TYPE IF EXISTS estado_pago CASCADE;
DROP TYPE IF EXISTS tipo_recordatorio CASCADE;
DROP TYPE IF EXISTS estado_recordatorio CASCADE;
DROP TYPE IF EXISTS operacion_auditoria CASCADE;
DROP TYPE IF EXISTS tipo_espacio CASCADE;
DROP TYPE IF EXISTS metodo_pago CASCADE;

-- Extensión necesaria para rangos de tiempo e índices GiST (Evitar solapamientos)
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- ==========================================
-- 1. TIPOS ENUM
-- ==========================================
CREATE TYPE estado_evento AS ENUM ('solicitado', 'en_revision', 'aprobado', 'programado', 'realizado', 'cancelado', 'rechazado');
CREATE TYPE estado_reserva AS ENUM ('confirmada', 'cancelada', 'asistencia_verificada');
CREATE TYPE estado_pago AS ENUM ('pendiente', 'completado', 'exento');
CREATE TYPE tipo_recordatorio AS ENUM ('email', 'notificacion_sistema');
CREATE TYPE estado_recordatorio AS ENUM ('pendiente', 'enviado', 'fallido');
CREATE TYPE operacion_auditoria AS ENUM ('INSERT', 'UPDATE', 'DELETE');
CREATE TYPE tipo_espacio AS ENUM ('biblioteca', 'auditorio', 'salon', 'laboratorio');
CREATE TYPE metodo_pago AS ENUM ('transferencia', 'efectivo', 'exento');

-- ==========================================
-- 2. TABLAS PRINCIPALES (Catálogos y Usuarios)
-- ==========================================
CREATE TABLE roles (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(50) NOT NULL UNIQUE,
    descripcion TEXT
);
COMMENT ON TABLE roles IS 'Roles de sistema (RBAC)';

CREATE TABLE usuarios (
    id BIGSERIAL PRIMARY KEY,
    nombre VARCHAR(100) NOT NULL,
    email VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255), 
    role_id INT NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    departamento VARCHAR(100),
    telefono VARCHAR(20),
    activo BOOLEAN NOT NULL DEFAULT true,
    oauth_provider VARCHAR(20) DEFAULT 'local',
    oauth_id VARCHAR(100),
    fecha_registro TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    -- NUEVO: Token de Firebase Cloud Messaging para enviar push notifications al celular
    fcm_token TEXT 
);
COMMENT ON TABLE usuarios IS 'Usuarios del sistema, incluyendo OAuth y tokens móviles';

CREATE TABLE categorias (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(50) NOT NULL UNIQUE,
    descripcion TEXT
);

CREATE TABLE espacios (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(100) NOT NULL,
    tipo tipo_espacio NOT NULL,
    capacidad INT NOT NULL CHECK (capacidad > 0),
    ubicacion TEXT,
    disponible BOOLEAN NOT NULL DEFAULT true,
    activo BOOLEAN NOT NULL DEFAULT true
);

-- ==========================================
-- 3. TABLAS TRANSACCIONALES
-- ==========================================
CREATE TABLE eventos (
    id BIGSERIAL PRIMARY KEY,
    titulo VARCHAR(150) NOT NULL,
    descripcion TEXT, 
    espacio_id INT NOT NULL REFERENCES espacios(id) ON DELETE RESTRICT,
    organizador_id BIGINT NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    aprobador_id BIGINT REFERENCES usuarios(id) ON DELETE SET NULL,
    fecha_inicio TIMESTAMP WITH TIME ZONE NOT NULL,
    fecha_fin TIMESTAMP WITH TIME ZONE NOT NULL,
    estado estado_evento NOT NULL DEFAULT 'solicitado',
    capacidad_maxima INT NOT NULL CHECK (capacidad_maxima > 0),
    imagen_url TEXT,
    observaciones TEXT,
    fecha_solicitud TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    fecha_aprobacion TIMESTAMP WITH TIME ZONE,
    fecha_creacion TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    fecha_actualizacion TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    calendar_event_id VARCHAR(255),
    CONSTRAINT chk_fechas_coherentes CHECK (fecha_fin > fecha_inicio)
);
COMMENT ON COLUMN eventos.descripcion IS 'Puede ser nulo inicialmente y generado luego por IA';

CREATE TABLE evento_categorias (
    evento_id BIGINT NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    categoria_id INT NOT NULL REFERENCES categorias(id) ON DELETE CASCADE,
    PRIMARY KEY (evento_id, categoria_id)
);

CREATE TABLE reservas (
    id BIGSERIAL PRIMARY KEY,
    evento_id BIGINT NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    usuario_id BIGINT NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    fecha_reserva TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    estado estado_reserva NOT NULL DEFAULT 'confirmada',
    codigo_qr TEXT UNIQUE,
    cantidad_entradas INT NOT NULL DEFAULT 1 CHECK (cantidad_entradas > 0)
);

CREATE TABLE pagos (
    id BIGSERIAL PRIMARY KEY,
    reserva_id BIGINT NOT NULL REFERENCES reservas(id) ON DELETE CASCADE,
    monto DECIMAL(10,2) NOT NULL CHECK (monto >= 0),
    metodo_pago metodo_pago NOT NULL DEFAULT 'exento',
    estado_pago estado_pago NOT NULL DEFAULT 'exento',
    fecha_pago TIMESTAMP WITH TIME ZONE,
    referencia TEXT
);

CREATE TABLE recordatorios (
    id BIGSERIAL PRIMARY KEY,
    evento_id BIGINT NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    destinatario_id BIGINT NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    tipo tipo_recordatorio NOT NULL DEFAULT 'email',
    contenido TEXT NOT NULL,
    fecha_envio_programada TIMESTAMP WITH TIME ZONE NOT NULL,
    fecha_envio_real TIMESTAMP WITH TIME ZONE,
    estado estado_recordatorio NOT NULL DEFAULT 'pendiente'
);

CREATE TABLE auditoria_log (
    id BIGSERIAL PRIMARY KEY,
    tabla_afectada TEXT NOT NULL,
    operacion operacion_auditoria NOT NULL,
    registro_id TEXT NOT NULL,
    usuario_modificador_id BIGINT REFERENCES usuarios(id) ON DELETE SET NULL,
    datos_anteriores JSONB,
    datos_nuevos JSONB,
    fecha_modificacion TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    ip_address INET
);

-- ==========================================
-- 4. REGLAS DE NEGOCIO Y TRIGGERS
-- ==========================================

-- A. Prevención de solapamientos (EXCLUDE GIST)
ALTER TABLE eventos ADD CONSTRAINT chk_sin_solapamiento 
EXCLUDE USING gist (
    espacio_id WITH =, 
    tsrange(fecha_inicio, fecha_fin) WITH &&
) WHERE (estado IN ('aprobado', 'programado'));

-- B. Límite de capacidad
CREATE OR REPLACE FUNCTION fn_check_capacidad_evento()
RETURNS TRIGGER AS $$
DECLARE
    capacidad_espacio INT;
BEGIN
    SELECT capacidad INTO capacidad_espacio FROM espacios WHERE id = NEW.espacio_id;
    
    IF NEW.capacidad_maxima > capacidad_espacio THEN
        RAISE EXCEPTION 'Capacidad excedida: El evento solicita %, pero el espacio solo admite %', NEW.capacidad_maxima, capacidad_espacio;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_check_capacidad
BEFORE INSERT OR UPDATE ON eventos
FOR EACH ROW EXECUTE FUNCTION fn_check_capacidad_evento();

-- C. Actualización de timestamp
CREATE OR REPLACE FUNCTION fn_update_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.fecha_actualizacion = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_eventos_updated_at
BEFORE UPDATE ON eventos
FOR EACH ROW EXECUTE FUNCTION fn_update_timestamp();

-- ==========================================
-- 5. AUDITORÍA AUTOMÁTICA
-- ==========================================
CREATE OR REPLACE FUNCTION fn_trigger_auditoria()
RETURNS TRIGGER AS $$
DECLARE
    v_usuario_id BIGINT;
    v_operacion operacion_auditoria;
    v_registro_id TEXT;
BEGIN
    BEGIN
        v_usuario_id := NULLIF(current_setting('myapp.current_user_id', true), '')::BIGINT;
    EXCEPTION WHEN OTHERS THEN
        v_usuario_id := NULL;
    END;

    IF (TG_OP = 'DELETE') THEN
        v_operacion := 'DELETE';
        v_registro_id := OLD.id::TEXT;
        INSERT INTO auditoria_log (tabla_afectada, operacion, registro_id, usuario_modificador_id, datos_anteriores)
        VALUES (TG_TABLE_NAME::TEXT, v_operacion, v_registro_id, v_usuario_id, row_to_json(OLD));
        RETURN OLD;
        
    ELSIF (TG_OP = 'UPDATE') THEN
        v_operacion := 'UPDATE';
        v_registro_id := NEW.id::TEXT;
        IF row_to_json(OLD) IS DISTINCT FROM row_to_json(NEW) THEN
            INSERT INTO auditoria_log (tabla_afectada, operacion, registro_id, usuario_modificador_id, datos_anteriores, datos_nuevos)
            VALUES (TG_TABLE_NAME::TEXT, v_operacion, v_registro_id, v_usuario_id, row_to_json(OLD), row_to_json(NEW));
        END IF;
        RETURN NEW;
        
    ELSIF (TG_OP = 'INSERT') THEN
        v_operacion := 'INSERT';
        v_registro_id := NEW.id::TEXT;
        INSERT INTO auditoria_log (tabla_afectada, operacion, registro_id, usuario_modificador_id, datos_nuevos)
        VALUES (TG_TABLE_NAME::TEXT, v_operacion, v_registro_id, v_usuario_id, row_to_json(NEW));
        RETURN NEW;
    END IF;
    
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Aplicación del trigger de auditoría
CREATE TRIGGER trg_auditoria_usuarios
AFTER INSERT OR UPDATE OR DELETE ON usuarios
FOR EACH ROW EXECUTE FUNCTION fn_trigger_auditoria();

CREATE TRIGGER trg_auditoria_eventos
AFTER INSERT OR UPDATE OR DELETE ON eventos
FOR EACH ROW EXECUTE FUNCTION fn_trigger_auditoria();

CREATE TRIGGER trg_auditoria_reservas
AFTER INSERT OR UPDATE OR DELETE ON reservas
FOR EACH ROW EXECUTE FUNCTION fn_trigger_auditoria();

CREATE TRIGGER trg_auditoria_pagos
AFTER INSERT OR UPDATE OR DELETE ON pagos
FOR EACH ROW EXECUTE FUNCTION fn_trigger_auditoria();

CREATE TRIGGER trg_auditoria_espacios
AFTER INSERT OR UPDATE OR DELETE ON espacios
FOR EACH ROW EXECUTE FUNCTION fn_trigger_auditoria();

-- ==========================================
-- 6. VISTAS DE NEGOCIO
-- ==========================================
CREATE OR REPLACE VIEW v_eventos_programados AS
SELECT 
    e.id,
    e.titulo,
    es.nombre AS espacio,
    u.nombre AS organizador,
    e.estado,
    e.fecha_inicio,
    e.capacidad_maxima,
    COUNT(r.id) AS reservas_confirmadas
FROM eventos e
JOIN espacios es ON e.espacio_id = es.id
JOIN usuarios u ON e.organizador_id = u.id
LEFT JOIN reservas r ON e.id = r.evento_id AND r.estado = 'confirmada'
WHERE e.estado IN ('programado', 'aprobado')
GROUP BY 
    e.id, e.titulo, es.nombre, u.nombre, e.estado, e.fecha_inicio, e.capacidad_maxima;

-- ==========================================
-- 7. ÍNDICES ESTRATÉGICOS
-- ==========================================
CREATE INDEX idx_usuarios_email ON usuarios(email);
CREATE INDEX idx_eventos_estado_fecha ON eventos(estado, fecha_inicio);
CREATE INDEX idx_eventos_espacio_fechas ON eventos(espacio_id, fecha_inicio, fecha_fin);
CREATE INDEX idx_eventos_organizador ON eventos(organizador_id);
CREATE INDEX idx_reservas_evento_estado ON reservas(evento_id, estado);
CREATE INDEX idx_reservas_usuario ON reservas(usuario_id);
CREATE INDEX idx_auditoria_tabla_fecha ON auditoria_log(tabla_afectada, fecha_modificacion);

-- ==========================================
-- 8. DATOS DE PRUEBA (SEED DATA - FACYT)
-- ==========================================

-- Roles
INSERT INTO roles (nombre, descripcion) VALUES 
('administrador', 'Acceso total, configuración del sistema'),
('aprobador', 'Coordinación Central de Cultura o Decanato'),
('organizador', 'Profesor, departamento o centro de estudiantes'),
('usuario', 'Estudiante o asistente general');

-- Espacios
INSERT INTO espacios (nombre, tipo, capacidad, ubicacion) VALUES 
('Biblioteca Central', 'biblioteca', 100, 'Edificio Central FaCyT, Piso 1'),
('Auditorio Principal', 'auditorio', 250, 'Planta Baja, cerca del Decanato');

-- Usuarios
INSERT INTO usuarios (nombre, email, password_hash, role_id, departamento) VALUES 
('Prof. Edgar', 'edgar.comp@uc.edu.ve', 'hash_simulado_123', (SELECT id FROM roles WHERE nombre = 'organizador'), 'Computación'),
('Coord. Cultura', 'cultura@uc.edu.ve', 'hash_simulado_123', (SELECT id FROM roles WHERE nombre = 'aprobador'), 'Coordinación Central'),
('Decano FaCyT', 'decanato@uc.edu.ve', 'hash_simulado_123', (SELECT id FROM roles WHERE nombre = 'administrador'), 'Decanato'),
('Estudiante Genérico', 'estudiante@uc.edu.ve', 'hash_simulado_123', (SELECT id FROM roles WHERE nombre = 'usuario'), 'Física');

-- Categorías
INSERT INTO categorias (nombre, descripcion) VALUES 
('Charla Técnica', 'Ponencias sobre temas específicos de carrera'),
('Taller Práctico', 'Actividades hands-on en laboratorios o aulas'),
('Reunión Institucional', 'Asambleas, consejos de facultad o reuniones del decanato');

-- Eventos (Con aprobador_id y fecha_aprobacion integrados)
INSERT INTO eventos (titulo, descripcion, espacio_id, organizador_id, aprobador_id, fecha_inicio, fecha_fin, estado, capacidad_maxima, fecha_aprobacion) VALUES 
('Introducción a Go y PostgreSQL', 'Taller sobre arquitecturas backend modernas', 
    (SELECT id FROM espacios WHERE nombre = 'Biblioteca Central'), 
    (SELECT id FROM usuarios WHERE email = 'edgar.comp@uc.edu.ve'), 
    (SELECT id FROM usuarios WHERE email = 'cultura@uc.edu.ve'),
    NOW() + INTERVAL '5 days', NOW() + INTERVAL '5 days 2 hours', 'aprobado', 50, NOW()),
    
('Asamblea de Facultad', 'Reunión general de profesores y autoridades', 
    (SELECT id FROM espacios WHERE nombre = 'Auditorio Principal'), 
    (SELECT id FROM usuarios WHERE email = 'decanato@uc.edu.ve'), 
    (SELECT id FROM usuarios WHERE email = 'cultura@uc.edu.ve'),
    NOW() + INTERVAL '10 days', NOW() + INTERVAL '10 days 4 hours', 'programado', 200, NOW());

-- Evento Categorías (Mapeo N:M)
INSERT INTO evento_categorias (evento_id, categoria_id) VALUES 
(1, (SELECT id FROM categorias WHERE nombre = 'Taller Práctico')),
(1, (SELECT id FROM categorias WHERE nombre = 'Charla Técnica')),
(2, (SELECT id FROM categorias WHERE nombre = 'Reunión Institucional'));

-- Reservas
INSERT INTO reservas (evento_id, usuario_id, codigo_qr, cantidad_entradas) VALUES 
(1, (SELECT id FROM usuarios WHERE email = 'estudiante@uc.edu.ve'), 'QR_MOCK_001', 1);

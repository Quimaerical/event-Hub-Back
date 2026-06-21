-- Schema for event-hub database

-- 1. Roles table
CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(50) NOT NULL UNIQUE
);

-- Seed basic roles
INSERT INTO roles (nombre) VALUES ('administrador') ON CONFLICT (nombre) DO NOTHING;
INSERT INTO roles (nombre) VALUES ('usuario') ON CONFLICT (nombre) DO NOTHING;

-- 2. Users table
CREATE TABLE IF NOT EXISTS usuarios (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(100) NOT NULL,
    email VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255), -- Nullable for OAuth users
    role_id INT NOT NULL REFERENCES roles(id),
    oauth_provider VARCHAR(20) DEFAULT 'local', -- 'local', 'google', 'github'
    oauth_id VARCHAR(100),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Index for searching users by email and oauth_id
CREATE INDEX IF NOT EXISTS idx_usuarios_email ON usuarios(email);
CREATE INDEX IF NOT EXISTS idx_usuarios_oauth ON usuarios(oauth_provider, oauth_id);

-- 3. Categories table
CREATE TABLE IF NOT EXISTS categorias (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(50) NOT NULL UNIQUE,
    descripcion TEXT
);

-- Seed some default categories
INSERT INTO categorias (nombre, descripcion) VALUES ('Tecnología', 'Eventos sobre desarrollo de software, hardware y tendencias tecnológicas') ON CONFLICT (nombre) DO NOTHING;
INSERT INTO categorias (nombre, descripcion) VALUES ('Música', 'Conciertos, festivales y recitales de música en vivo') ON CONFLICT (nombre) DO NOTHING;
INSERT INTO categorias (nombre, descripcion) VALUES ('Negocios', 'Conferencias de negocios, emprendimiento y networking') ON CONFLICT (nombre) DO NOTHING;
INSERT INTO categorias (nombre, descripcion) VALUES ('Arte', 'Exposiciones de arte, talleres de pintura y galerías') ON CONFLICT (nombre) DO NOTHING;

-- 4. Events table
CREATE TABLE IF NOT EXISTS eventos (
    id SERIAL PRIMARY KEY,
    titulo VARCHAR(150) NOT NULL,
    descripcion TEXT NOT NULL,
    fecha TIMESTAMP NOT NULL,
    ubicacion VARCHAR(255) NOT NULL,
    creador_id INT NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Index for indexed searches on events (by location, date and title)
CREATE INDEX IF NOT EXISTS idx_eventos_fecha ON eventos(fecha);
CREATE INDEX IF NOT EXISTS idx_eventos_titulo ON eventos(titulo);

-- 5. Event Categories table (Many-to-Many relation)
CREATE TABLE IF NOT EXISTS evento_categorias (
    evento_id INT NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    categoria_id INT NOT NULL REFERENCES categorias(id) ON DELETE CASCADE,
    PRIMARY KEY (evento_id, categoria_id)
);

-- Indexes for join table performance
CREATE INDEX IF NOT EXISTS idx_evento_categorias_evento ON evento_categorias(evento_id);
CREATE INDEX IF NOT EXISTS idx_evento_categorias_categoria ON evento_categorias(categoria_id);

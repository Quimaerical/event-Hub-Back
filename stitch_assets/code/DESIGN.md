# Especificaciones de Diseño - Event Hub

Este documento define la guía de estilos, los tokens de diseño, la paleta de colores y las especificaciones UX/UI del frontend de **Event Hub** para su integración directa con diseños exportados de **Google Stitch**.

---

## 1. Paleta de Colores (Tokens CSS & Tailwind)

La identidad visual del proyecto se basa en una paleta minimalista y de alto contraste, centrada en tonos azul cielo, verdemar, blanco y negro.

| Token | Propósito | Valor Hexadecimal | Clases Tailwind (Equivalente) |
| :--- | :--- | :--- | :--- |
| **Sky Blue** | Acento principal, enlaces, enfoque | `#0ea5e9` | `text-sky-500` / `bg-sky-500` |
| **Sea Green** | Acento secundario, badges, éxito | `#0d9488` | `text-teal-600` / `bg-teal-600` |
| **White** | Fondo claro, texto en modo oscuro | `#ffffff` | `bg-white` / `text-white` |
| **Black** | Fondo oscuro, texto en modo claro | `#020617` | `bg-slate-950` / `text-slate-950` |
| **Muted Slate** | Textos secundarios, bordes sutiles | `#64748b` | `text-slate-500` / `border-slate-800` |

---

## 2. Tipografía y Estructura Visual

*   **Tipografía**: **Outfit** (de Google Fonts). Se utiliza sin serifa con pesos de `300` (light), `400` (regular), `500` (medium), `600` (semibold) y `700` (bold).
*   **Radios de Borde (Rounded corners)**: Estilo moderno con esquinas suaves. Se recomiendan clases como `rounded-2xl` (1rem / 16px) para tarjetas y paneles, y `rounded-xl` (0.75rem / 12px) para botones y campos de entrada.
*   **Estilo Moderno Minimalista**:
    *   **Modo Oscuro**: Fondo negro puro (`bg-black` o `bg-slate-950`) combinado con paneles semitransparentes translúcidos (efecto *glassmorphism* con `backdrop-blur-md` y bordes finos de color blanco al 5% o 10% de opacidad).
    *   **Modo Claro**: Fondo blanco puro (`bg-white` o `bg-slate-50`) con sombras muy suaves y bordes finos grises (`border-slate-200/50`).

---

## 3. Páginas y Componentes Clave

### A. Estructura Global y Navbar / Footer
*   **Navbar**: Sticky en la parte superior (`sticky top-0 z-40`). Contiene el logo a la izquierda, enlaces a la cartelera y creación a la mitad, y estado de autenticación (iniciar sesión/registrarse o el usuario logueado con botón de cerrar sesión) a la derecha. Debe incluir el botón alternador de Modo Claro / Modo Oscuro.
*   **Footer**: Sencillo, centrado y con textos secundarios (`text-slate-500` / `text-slate-400`).

### B. Cartelera Principal (Dashboard)
*   **Buscador**: Campo de texto estilizado que incluye un ícono de lupa y conserva el estado de filtros activos.
*   **Filtros de Categoría**: Lista horizontal de badges/pills interactivos. El pill seleccionado tiene fondo sólido de acento (`bg-sky-500` o `bg-teal-600`) y los inactivos tienen fondo transparente con bordes sutiles.
*   **Cuadrícula de Eventos (Event Grid)**: Tarjetas responsivas (1 columna en móvil, 2 en tablet, 3 en desktop) que muestran categorías, título del evento, descripción abreviada, fecha formateada, ubicación y nombre del creador.

### C. Formularios (Login, Registro y Crear Evento)
*   Contenedores limpios, centrados y estrechos (máximo `max-w-md` para Login/Registro y `max-w-2xl` para Crear Evento).
*   Campos con etiquetas claras de tamaño reducido (`text-xs font-semibold uppercase tracking-wider`) y transiciones de color de borde al enfocarse (`focus:border-sky-500 focus:ring-1 focus:ring-sky-500`).
*   **Funcionalidad Asíncrona de Gemini**: El formulario de "Crear Evento" debe contar con un botón llamativo de **"Sugerir descripción con IA"** al lado de la descripción. Este botón realiza una petición asíncrona por detrás para rellenar la descripción a partir del título y la ubicación.

---

## 4. Estilos de Modo Oscuro y Claro (Tailwind)

Se implementa a través de la estrategia de clase `class="dark"` en la etiqueta HTML. Todos los componentes de Stitch deben incluir la variante `dark:`:

*   **Fondo de Página**: `bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-slate-100`
*   **Paneles y Tarjetas**:
    *   *Modo Claro*: `bg-white border border-slate-200/80 shadow-sm hover:shadow-md`
    *   *Modo Oscuro*: `bg-slate-900/40 border border-white/5 backdrop-blur-md hover:bg-slate-900/60`
*   **Campos de Entrada (Inputs)**:
    *   *Modo Claro*: `bg-slate-100/80 border-slate-200 text-slate-900 focus:bg-white`
    *   *Modo Oscuro*: `bg-slate-950/60 border-slate-800 text-white focus:bg-slate-950`

---

## 5. Accesibilidad e Interacciones (UX)
*   **Micro-interacciones**: Transiciones suaves (`transition duration-200 ease-in-out`) en botones, enlaces y tarjetas.
*   **Hover states**: Efecto de elevación sutil en tarjetas (`hover:-translate-y-1 hover:shadow-lg`) y cambios de opacidad/color en botones primarios.

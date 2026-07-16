# Design System - Event Hub

## Style Guidelines

The brand personality is professional, modern, and high-tech, designed to evoke a sense of reliability and excitement for event discovery and management. The design style follows a **Modern Corporate** aesthetic with strong **Glassmorphic** influences, particularly in the dark mode implementation.

The UI is characterized by a high-contrast relationship between deep slates and vibrant sky blues. It utilizes generous whitespace, translucent layering, and soft architectural shadows to create a sophisticated, tech-forward environment that remains accessible and functional.

### Layout & Spacing
- **Desktop:** 12-column grid with a 1.5rem (24px) gutter. Content is centered with a max-width of 1280px.
- **Mobile:** Single column layout with 1rem (16px) side margins.
- **Rhythm:** Spacing follows a strict 4px/8px baseline to ensure consistent vertical alignment between components.

The Navbar is fixed to the top of the viewport with a high z-index, ensuring global navigation and the theme toggle are always accessible.

### Elevation & Depth
Depth is handled differently across color modes to maximize the "Modern Minimalist" aesthetic:
- **Dark Mode:** Utilizes **Glassmorphism**. Surfaces are semi-transparent slates (`bg-slate-900/40`) with a `backdrop-blur-md` effect. Borders are thin and light (5-10% white opacity) to define edges without adding bulk.
- **Light Mode:** Uses **Tonal Layers** and soft ambient shadows. Surfaces are pure white with very subtle slate borders (`border-slate-200/50`) and low-opacity shadows to create a clean, floating effect.

### Components

#### Buttons
- **Primary:** Sky Blue background with white text. High-contrast, bold, and uses the `rounded-xl` shape.
- **Secondary:** Sea Green background, used for success-oriented actions or positive badges.
- **Ghost:** Transparent background with Sky Blue text or white/slate borders, used for less prominent actions.

#### Inputs
- **Light Mode:** Soft slate background (`bg-slate-100/80`) that transitions to white on focus.
- **Dark Mode:** Deep slate translucent background (`bg-slate-950/60`) with a focus state that increases opacity.

#### Cards
Cards are the primary content vessel. In Dark Mode, they must include the backdrop blur and fine white border to maintain the glassmorphic style. Hover states should subtly increase the background opacity or shadow intensity to provide tactile feedback.

#### Navigation Bar
The navbar should be a glassmorphic element in both modes, using `sticky top-0` and `backdrop-blur` to allow content to scroll underneath while remaining visible. It must house the theme toggle prominently on the right side.

---

## Design Markdown (designMd)

```yaml
---
name: Event Hub
colors:
  surface: '#0f1418'
  surface-dim: '#0f1418'
  surface-bright: '#353a3e'
  surface-container-lowest: '#0a0f13'
  surface-container-low: '#171c20'
  surface-container: '#1b2024'
  surface-container-high: '#252b2f'
  surface-container-highest: '#30353a'
  on-surface: '#dee3e9'
  on-surface-variant: '#bec8d2'
  inverse-surface: '#dee3e9'
  inverse-on-surface: '#2c3135'
  outline: '#88929b'
  outline-variant: '#3e4850'
  surface-tint: '#89ceff'
  primary: '#89ceff'
  on-primary: '#00344d'
  primary-container: '#0ea5e9'
  on-primary-container: '#003751'
  inverse-primary: '#006591'
  secondary: '#6bd8cb'
  on-secondary: '#003732'
  secondary-container: '#29a195'
  on-secondary-container: '#00302b'
  tertiary: '#ffb86e'
  on-tertiary: '#492900'
  tertiary-container: '#de8712'
  on-tertiary-container: '#4d2b00'
  error: '#ffb4ab'
  on-error: '#690005'
  error-container: '#93000a'
  on-error-container: '#ffdad6'
  primary-fixed: '#c9e6ff'
  primary-fixed-dim: '#89ceff'
  on-primary-fixed: '#001e2f'
  on-primary-fixed-variant: '#004c6e'
  secondary-fixed: '#89f5e7'
  secondary-fixed-dim: '#6bd8cb'
  on-secondary-fixed: '#00201d'
  on-secondary-fixed-variant: '#005049'
  tertiary-fixed: '#ffdcbd'
  tertiary-fixed-dim: '#ffb86e'
  on-tertiary-fixed: '#2c1600'
  on-tertiary-fixed-variant: '#693c00'
  background: '#0f1418'
  on-background: '#dee3e9'
  surface-variant: '#30353a'
  bg-light: '#f8fafc'
  bg-dark: '#020617'
  muted-slate: '#64748b'
typography:
  display-lg:
    fontFamily: Outfit
    fontSize: 48px
    fontWeight: '700'
    lineHeight: '1.1'
  display-lg-mobile:
    fontFamily: Outfit
    fontSize: 36px
    fontWeight: '700'
    lineHeight: '1.2'
  headline-lg:
    fontFamily: Outfit
    fontSize: 32px
    fontWeight: '600'
    lineHeight: '1.25'
  headline-md:
    fontFamily: Outfit
    fontSize: 24px
    fontWeight: '600'
    lineHeight: '1.3'
  body-lg:
    fontFamily: Outfit
    fontSize: 18px
    fontWeight: '400'
    lineHeight: '1.6'
  body-md:
    fontFamily: Outfit
    fontSize: 16px
    fontWeight: '400'
    lineHeight: '1.5'
  label-md:
    fontFamily: Outfit
    fontSize: 14px
    fontWeight: '500'
    lineHeight: '1.4'
    letterSpacing: 0.025em
  label-sm:
    fontFamily: Outfit
    fontSize: 12px
    fontWeight: '600'
    lineHeight: '1.2'
    letterSpacing: 0.05em
rounded:
  sm: 0.25rem
  DEFAULT: 0.5rem
  md: 0.75rem
  lg: 1rem
  xl: 1.5rem
  full: 9999px
spacing:
  container-max: 1280px
  gutter: 1.5rem
  margin-mobile: 1rem
  margin-desktop: 2rem
---

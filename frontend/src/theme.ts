import '@fontsource-variable/geist'
import '@fontsource-variable/jetbrains-mono'
import '@fontsource-variable/space-grotesk'
import { alpha, createTheme } from '@mui/material/styles'

export const space = {
  canvas: '#0A0E17',
  panel: '#111B2B',
  elevated: '#19263A',
  hover: '#203249',
  primary: '#38BDF8',
  secondary: '#A5B4FC',
  text: '#E6EDF7',
  muted: '#A8B8CC',
  divider: '#2B405A',
  outline: '#58718F',
  success: '#34D399',
  warning: '#FBBF24',
  error: '#F87171',
} as const

export const fonts = {
  heading: '"Space Grotesk Variable", "Space Grotesk", sans-serif',
  body: '"Geist Variable", Geist, system-ui, sans-serif',
  technical:
    '"JetBrains Mono Variable", "JetBrains Mono", ui-monospace, monospace',
} as const

const focusRing = {
  outline: `2px solid ${space.primary}`,
  outlineOffset: 2,
}

export const theme = createTheme({
  palette: {
    mode: 'dark',
    primary: { main: space.primary, contrastText: space.canvas },
    secondary: { main: space.secondary, contrastText: space.canvas },
    success: { main: space.success, contrastText: space.canvas },
    warning: { main: space.warning, contrastText: space.canvas },
    error: { main: space.error, contrastText: space.canvas },
    info: { main: space.primary, contrastText: space.canvas },
    background: { default: space.canvas, paper: space.panel },
    text: { primary: space.text, secondary: space.muted },
    divider: space.divider,
  },
  shape: { borderRadius: 4 },
  typography: {
    htmlFontSize: 16,
    fontFamily: fonts.body,
    fontSize: 14,
    body1: { fontSize: '0.875rem', lineHeight: 1.5 },
    body2: { fontSize: '0.875rem', lineHeight: 1.45 },
    h1: { fontFamily: fonts.heading, fontWeight: 650 },
    h2: { fontFamily: fonts.heading, fontWeight: 650 },
    h3: { fontFamily: fonts.heading, fontWeight: 650 },
    h4: { fontFamily: fonts.heading, fontWeight: 650 },
    h5: { fontFamily: fonts.heading, fontWeight: 650 },
    h6: { fontFamily: fonts.heading, fontWeight: 650 },
    subtitle1: { fontFamily: fonts.heading, fontWeight: 600 },
    subtitle2: { fontFamily: fonts.heading, fontWeight: 600 },
    overline: { fontWeight: 650, letterSpacing: '0.1em' },
    button: { fontWeight: 650, letterSpacing: '0.01em', textTransform: 'none' },
  },
  shadows: [
    'none',
    `0 8px 24px ${alpha('#000000', 0.2)}`,
    `0 12px 32px ${alpha('#000000', 0.25)}`,
    ...Array(22).fill(`0 16px 40px ${alpha('#000000', 0.3)}`),
  ] as ReturnType<typeof createTheme>['shadows'],
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        ':root': { colorScheme: 'dark' },
        body: { minWidth: 320, backgroundColor: space.canvas },
        '::selection': { backgroundColor: alpha(space.primary, 0.35) },
        '*:focus-visible': focusRing,
        '@media (prefers-reduced-motion: reduce)': {
          '*, *::before, *::after': {
            scrollBehavior: 'auto !important',
            transitionDuration: '0.01ms !important',
            animationDuration: '0.01ms !important',
            animationIterationCount: '1 !important',
          },
        },
      },
    },
    MuiAppBar: {
      styleOverrides: {
        root: {
          backgroundColor: alpha(space.panel, 0.96),
          borderBottom: `1px solid ${space.divider}`,
          backgroundImage: 'none',
        },
      },
    },
    MuiPaper: {
      styleOverrides: {
        root: { backgroundImage: 'none' },
        rounded: { borderRadius: 8 },
        outlined: { borderColor: space.divider },
      },
    },
    MuiButton: {
      styleOverrides: {
        root: { borderRadius: 4 },
        containedPrimary: {
          '&:hover': { backgroundColor: '#70D0F9' },
        },
      },
    },
    MuiOutlinedInput: {
      styleOverrides: {
        root: {
          backgroundColor: space.canvas,
          '& .MuiOutlinedInput-notchedOutline': { borderColor: space.outline },
          '&:hover .MuiOutlinedInput-notchedOutline': {
            borderColor: space.primary,
          },
          '&.Mui-focused .MuiOutlinedInput-notchedOutline': {
            borderColor: space.primary,
            borderWidth: 2,
          },
          '&.Mui-disabled': { backgroundColor: alpha(space.canvas, 0.55) },
          '&.Mui-disabled .MuiOutlinedInput-notchedOutline': {
            borderColor: space.divider,
          },
        },
      },
    },
    MuiMenu: {
      styleOverrides: {
        paper: {
          backgroundColor: space.elevated,
          border: `1px solid ${space.divider}`,
        },
      },
    },
    MuiMenuItem: {
      styleOverrides: {
        root: {
          '&:hover': { backgroundColor: space.hover },
          '&.Mui-selected': { backgroundColor: alpha(space.primary, 0.16) },
        },
      },
    },
    MuiListItemButton: {
      styleOverrides: {
        root: {
          borderRadius: 4,
          '&:hover': { backgroundColor: space.hover },
          '&.Mui-selected': {
            color: space.primary,
            backgroundColor: alpha(space.primary, 0.14),
            boxShadow: `inset 2px 0 ${space.primary}`,
          },
          '&.Mui-selected:hover': {
            backgroundColor: alpha(space.primary, 0.2),
          },
        },
      },
    },
    MuiTableCell: {
      styleOverrides: {
        root: { borderColor: space.divider, padding: '8px 12px' },
        head: {
          color: space.muted,
          backgroundColor: space.elevated,
          fontSize: '0.75rem',
          fontWeight: 650,
          letterSpacing: '0.04em',
          textTransform: 'uppercase',
        },
      },
    },
    MuiTableRow: {
      styleOverrides: {
        root: { '&.MuiTableRow-hover:hover': { backgroundColor: space.hover } },
      },
    },
    MuiDialog: {
      styleOverrides: {
        paper: {
          backgroundColor: space.elevated,
          border: `1px solid ${space.outline}`,
          borderRadius: 8,
        },
      },
    },
    MuiDialogTitle: {
      styleOverrides: {
        root: {
          fontFamily: fonts.heading,
          borderBottom: `1px solid ${space.divider}`,
        },
      },
    },
    MuiDialogActions: {
      styleOverrides: { root: { borderTop: `1px solid ${space.divider}` } },
    },
    MuiTabs: {
      styleOverrides: { root: { borderBottom: `1px solid ${space.divider}` } },
    },
    MuiTab: {
      styleOverrides: {
        root: { minHeight: 44, textTransform: 'none', fontWeight: 650 },
      },
    },
    MuiChip: {
      styleOverrides: {
        root: {
          fontFamily: fonts.technical,
          fontSize: '0.75rem',
          fontWeight: 650,
        },
      },
    },
    MuiAlert: {
      styleOverrides: {
        root: {
          border: '1px solid',
          borderColor: 'currentColor',
          backgroundImage: 'none',
        },
        standardSuccess: { backgroundColor: alpha(space.success, 0.1) },
        standardWarning: { backgroundColor: alpha(space.warning, 0.1) },
        standardError: { backgroundColor: alpha(space.error, 0.1) },
        standardInfo: { backgroundColor: alpha(space.primary, 0.1) },
      },
    },
    MuiCircularProgress: { styleOverrides: { root: { color: space.primary } } },
  },
})

export const shellBackground = {
  backgroundColor: space.canvas,
  backgroundImage: `radial-gradient(circle at 18% 12%, ${alpha(space.primary, 0.1)}, transparent 28%), radial-gradient(circle at 86% 3%, ${alpha(space.secondary, 0.08)}, transparent 24%)`,
} as const

export const technicalText = {
  fontFamily: fonts.technical,
  fontVariantNumeric: 'tabular-nums',
  overflowWrap: 'anywhere',
} as const

export const subtleStripe = alpha(space.text, 0.018)
export const editorInset = `inset 0 1px 0 ${alpha(space.text, 0.03)}`

import { extendTheme, type ThemeConfig, type StyleFunctionProps } from '@chakra-ui/react';

const config: ThemeConfig = {
  initialColorMode: 'light',
  useSystemColorMode: false,
  disableTransitionOnChange: false,
};

const theme = extendTheme({
  config,
  colors: {
    brand: {
      50: '#E3F2FD',
      100: '#BBDEFB', 
      200: '#90CAF9',
      300: '#64B5F6',
      400: '#42A5F5',
      500: '#2196F3', // Primary blue color like in the design
      600: '#1E88E5',
      700: '#1976D2',
      800: '#1565C0',
      900: '#0D47A1',
    },
  },
  fonts: {
    heading: 'Inter, system-ui, sans-serif',
    body: 'Inter, system-ui, sans-serif',
  },
  styles: {
    global: (props: StyleFunctionProps) => ({
      body: {
        bg: props.colorMode === 'dark' ? 'var(--bg-primary)' : 'var(--bg-secondary)',
        color: props.colorMode === 'dark' ? 'var(--text-primary)' : 'var(--text-primary)',
        transition: 'background-color 0.3s ease, color 0.3s ease',
      },
      '*::placeholder': {
        color: props.colorMode === 'dark' ? 'var(--text-secondary)' : 'var(--text-secondary)',
      },
      '*, *::before, &::after': {
        borderColor: props.colorMode === 'dark' ? 'var(--border-color)' : 'var(--border-color)',
      },
    }),
  },
  components: {
    Button: {
      defaultProps: {
        colorScheme: 'brand',
      },
      variants: {
        solid: {
          fontWeight: 'medium',
          _hover: {
            transform: 'translateY(-1px)',
            boxShadow: 'lg',
          },
        },
      },
    },
    Card: {
      baseStyle: (props: StyleFunctionProps) => ({
        container: {
          borderRadius: 'lg',
          overflow: 'hidden',
          border: '1px',
          borderColor: props.colorMode === 'dark' ? 'var(--border-color)' : 'gray.200',
          bg: props.colorMode === 'dark' ? 'var(--bg-secondary)' : 'white',
          boxShadow: props.colorMode === 'dark' ? 'var(--shadow)' : 'sm',
          transition: 'all 0.3s ease',
        },
      }),
    },
    Table: {
      variants: {
        simple: (props: StyleFunctionProps) => ({
          th: {
            fontWeight: '500',
            fontSize: 'sm',
            color: props.colorMode === 'dark' ? 'var(--text-primary)' : 'gray.600',
            textTransform: 'none',
            borderColor: props.colorMode === 'dark' ? 'var(--border-color)' : 'gray.200',
            bg: props.colorMode === 'dark' ? 'var(--bg-tertiary)' : 'gray.50',
          },
          td: {
            borderColor: props.colorMode === 'dark' ? 'var(--border-color)' : 'gray.200',
            fontSize: 'sm',
            color: props.colorMode === 'dark' ? 'var(--text-primary)' : 'gray.800',
          },
          tbody: {
            tr: {
              _hover: {
                bg: props.colorMode === 'dark' ? 'var(--bg-tertiary)' : 'gray.50',
              },
            },
          },
        }),
      },
    },
    Heading: {
      baseStyle: {
        fontWeight: '600',
        letterSpacing: 'tight',
      },
    },
    Text: {
      baseStyle: {
        lineHeight: 'base',
      },
    },
    Alert: {
      variants: {
        subtle: (props: StyleFunctionProps) => {
          const isLightMode = props.colorMode === 'light';
          const isSuccess = props.status === 'success' || props.colorScheme === 'green';

          if (isLightMode && isSuccess) {
            return {
              container: {
                bg: 'green.50',
                color: 'green.800',
                borderLeftWidth: '4px',
                borderLeftColor: 'green.500',
                boxShadow: 'sm',
              },
              title: {
                color: 'green.800',
              },
              description: {
                color: 'green.700',
              },
              icon: {
                color: 'green.600',
              },
            };
          }

          return {};
        },
      },
    },
  },
});

export default theme;

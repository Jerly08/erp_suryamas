'use client';

import React from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useTheme } from '@/contexts/SimpleThemeContext';
import { useTranslation } from '@/hooks/useTranslation';
import SimpleThemeToggle from '@/components/common/SimpleThemeToggle';
import UnifiedNotifications from '@/components/notification/UnifiedNotifications';
import { formatRoleForApproval } from '@/utils/roles';
import { getUserDisplayName } from '@/utils/userDisplay';
import { useBranding } from '@/contexts/BrandingContext';
import { getScaledLogoSize } from '@/config/branding';
import {
  Flex,
  Image,
  Text,
  IconButton,
  HStack,
  VStack,
  Avatar,
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  Button,
  Tooltip,
  useColorMode,
  useColorModeValue,
} from '@chakra-ui/react';
import { FiMenu, FiChevronDown, FiChevronsRight, FiLogOut } from 'react-icons/fi';

interface SimpleNavbarProps {
  onMenuClick?: () => void;
  isMenuOpen?: boolean;
  onSidebarToggle?: () => void;
  isSidebarCollapsed?: boolean;
}

const SimpleNavbar: React.FC<SimpleNavbarProps> = ({
  onMenuClick,
  isMenuOpen = false,
  onSidebarToggle,
  isSidebarCollapsed = false,
}) => {
  const { user, logout } = useAuth();
  const { theme } = useTheme();
  const { t } = useTranslation();
  const { colorMode, setColorMode } = useColorMode();
  const { companyName, logoUrl, logoScale } = useBranding();
  const displayName = getUserDisplayName(user, { fallback: 'User', allowRoleFallback: false });
  const roleLabel = formatRoleForApproval(user?.role);
  
  // Dynamic colors based on theme
  const bgColor = useColorModeValue('white', 'var(--bg-secondary)');
  const borderColor = useColorModeValue('gray.200', 'var(--border-color)');
  const textColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const subtextColor = useColorModeValue('gray.500', 'var(--text-secondary)');

  // Sync Chakra UI color mode with our custom theme
  React.useEffect(() => {
    if (colorMode !== theme) {
      setColorMode(theme);
    }
  }, [theme, colorMode, setColorMode]);

  return (
    <Flex
      h={{ base: '14', md: '16' }}
      alignItems="center"
      justifyContent="space-between"
      px={{ base: 2, sm: 3, md: 4 }}
      gap={2}
      bg={bgColor}
      borderBottom="1px"
      borderColor={borderColor}
      position="sticky"
      top={0}
      zIndex={1000}
      className="navbar"
      transition="all 0.3s ease"
      minW={0}
    >
      {/* Mobile menu button */}
      {onMenuClick && (
        <IconButton
          display={{ base: 'flex', lg: 'none' }}
          onClick={onMenuClick}
          variant="outline"
          aria-label={isMenuOpen ? t('navigation.navbar.closeMenu') : t('navigation.navbar.openMenu')}
          icon={<FiMenu />}
          size="sm"
          flexShrink={0}
        />
      )}

      {onSidebarToggle && isSidebarCollapsed && (
        <Tooltip label="Buka sidebar" placement="bottom">
          <IconButton
            display={{ base: 'none', lg: 'flex' }}
            onClick={onSidebarToggle}
            variant="ghost"
            aria-label="Buka sidebar"
            icon={<FiChevronsRight />}
            size="sm"
            flexShrink={0}
          />
        </Tooltip>
      )}

      {/* Brand - visible on mobile when no sidebar */}
      <Image
        src={logoUrl}
        alt={companyName}
        h={{
          base: getScaledLogoSize(28, logoScale),
          md: getScaledLogoSize(32, logoScale),
        }}
        maxW={{ base: '112px', md: '132px' }}
        objectFit="contain"
        display={{ base: onMenuClick ? 'none' : 'block', lg: 'none' }}
        flexShrink={0}
      />

      {/* Spacer for desktop */}
      <div style={{ flex: 1, minWidth: 0 }} />

      {/* Right side */}
      <HStack spacing={{ base: 1, sm: 2, md: 4 }} minW={0} flexShrink={0}>
        {/* Notifications */}
        <UnifiedNotifications />
        
        {/* Theme Toggle */}
        <SimpleThemeToggle />

        {/* User menu */}
        {user && (
          <Menu>
            <MenuButton
              as={Button}
              variant="ghost"
              rightIcon={<FiChevronDown />}
              size={{ base: 'sm', md: 'md' }}
              px={{ base: 1, sm: 2, md: 4 }}
              minW={0}
            >
              <HStack spacing={{ base: 1, md: 2 }} minW={0}>
                <Avatar size={{ base: 'xs', md: 'sm' }} name={displayName} bg="brand.500" flexShrink={0} />
                <VStack spacing={0} align="start" display={{ base: 'none', md: 'flex' }}>
                  <Text fontSize="sm" fontWeight="medium" color={textColor} noOfLines={1}>
                    {displayName}
                  </Text>
                  <Text fontSize="xs" color={subtextColor} noOfLines={1}>
                    {roleLabel}
                  </Text>
                </VStack>
              </HStack>
            </MenuButton>
            <MenuList zIndex={9999}>
              <MenuItem icon={<FiLogOut />} onClick={logout}>
                {t('navigation.navbar.signOut')}
              </MenuItem>
            </MenuList>
          </Menu>
        )}
      </HStack>
    </Flex>
  );
};

export default SimpleNavbar;

'use client';

import React from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useTheme } from '@/contexts/SimpleThemeContext';
import { useTranslation } from '@/hooks/useTranslation';
import SimpleThemeToggle from '@/components/common/SimpleThemeToggle';
import UnifiedNotifications from '@/components/notification/UnifiedNotifications';
import { formatRoleForApproval } from '@/utils/roles';
import { getUserDisplayName } from '@/utils/userDisplay';
import {
  Flex,
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
  useColorMode,
  useColorModeValue,
} from '@chakra-ui/react';
import { FiMenu, FiChevronDown, FiLogOut } from 'react-icons/fi';

interface SimpleNavbarProps {
  onMenuClick?: () => void;
  isMenuOpen?: boolean;
}

const SimpleNavbar: React.FC<SimpleNavbarProps> = ({ onMenuClick, isMenuOpen = false }) => {
  const { user, logout } = useAuth();
  const { theme } = useTheme();
  const { t } = useTranslation();
  const { colorMode, setColorMode } = useColorMode();
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
      h="16"
      alignItems="center"
      justifyContent="space-between"
      px={4}
      bg={bgColor}
      borderBottom="1px"
      borderColor={borderColor}
      position="sticky"
      top={0}
      zIndex={1000}
      className="navbar"
      transition="all 0.3s ease"
    >
      {/* Mobile menu button */}
      {onMenuClick && (
        <IconButton
          display={{ base: 'flex', md: 'none' }}
          onClick={onMenuClick}
          variant="outline"
          aria-label={isMenuOpen ? t('navigation.navbar.closeMenu') : t('navigation.navbar.openMenu')}
          icon={<FiMenu />}
        />
      )}

      {/* Title - visible on mobile when no sidebar */}
      <Text fontSize="lg" fontWeight="semibold" color={textColor} display={{ base: onMenuClick ? 'none' : 'block', md: 'none' }}>
        {t('navigation.navbar.systemTitle')}
      </Text>

      {/* Spacer for desktop */}
      <div style={{ flex: 1 }} />

      {/* Right side */}
      <HStack spacing={4}>
        {/* Notifications */}
        <UnifiedNotifications />
        
        {/* Theme Toggle */}
        <SimpleThemeToggle />

        {/* User menu */}
        {user && (
          <Menu>
            <MenuButton as={Button} variant="ghost" rightIcon={<FiChevronDown />}>
              <HStack spacing={2}>
                <Avatar size="sm" name={displayName} bg="brand.500" />
                <VStack spacing={0} align="start" display={{ base: 'none', md: 'flex' }}>
                  <Text fontSize="sm" fontWeight="medium" color={textColor}>
                    {displayName}
                  </Text>
                  <Text fontSize="xs" color={subtextColor}>
                    {roleLabel}
                  </Text>
                </VStack>
              </HStack>
            </MenuButton>
            <MenuList>
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

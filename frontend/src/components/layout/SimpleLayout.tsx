'use client';

import React from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useAuthService } from '@/hooks/useAuthService';
import { UserRole } from '@/contexts/AuthContext';
import { normalizeRoles } from '@/utils/roles';
import SimpleNavbar from './SimpleNavbar';
import Sidebar from './SidebarNew';
import ProtectedRoute from '../auth/ProtectedRoute';
import { Box, useDisclosure, useBreakpointValue, useColorModeValue } from '@chakra-ui/react';

interface SimpleLayoutProps {
  children: React.ReactNode;
  title?: string;
  allowedRoles?: (UserRole | string)[];
  showSidebar?: boolean;
  requireAuth?: boolean;
}

const SimpleLayout: React.FC<SimpleLayoutProps> = ({ 
  children, 
  title: _title,
  allowedRoles = [], 
  showSidebar = true,
  requireAuth = true 
}) => {
  useAuthService(); // Setup unauthorized handler
  const { user } = useAuth();
  const { isOpen, onOpen, onClose } = useDisclosure();
  const normalizedAllowed = normalizeRoles(allowedRoles as string[]);
  const isMobile = useBreakpointValue({ base: true, lg: false }, { fallback: 'lg' });
  
  // Dynamic background colors based on theme
  const bgColor = useColorModeValue('var(--bg-secondary)', 'var(--bg-primary)');

  // If authentication is not required, render content directly
  if (!requireAuth) {
    return (
      <Box minH="100vh" bg={bgColor} transition="background-color 0.3s ease">
        {children}
      </Box>
    );
  }

  return (
    <ProtectedRoute allowedRoles={normalizedAllowed as any}>
      <Box minH="100vh" bg={bgColor} transition="background-color 0.3s ease">
        {/* Sidebar for desktop */}
        {showSidebar && !isMobile && user && (
          <Sidebar 
            isOpen
            onClose={onClose} 
            display={{ base: 'none', lg: 'block' }}
            width={60}
            collapsed={false}
            onToggleCollapse={() => {}}
            variant="fixed"
          />
        )}

        {/* Mobile drawer sidebar */}
        {showSidebar && user && (
          <Sidebar
            isOpen={isOpen}
            onClose={onClose}
            display={{ base: 'block', lg: 'none' }}
            width={60}
            collapsed={false}
            onToggleCollapse={() => {}}
            variant="drawer"
          />
        )}

        {/* Main content */}
        <Box ml={{ base: 0, lg: showSidebar ? 60 : 0 }} minW={0}>
          {/* Top navigation bar - always show if user is authenticated */}
          {user && (
            <SimpleNavbar 
              onMenuClick={showSidebar ? onOpen : undefined} 
              isMenuOpen={isOpen} 
            />
          )}
          
          {/* Page content */}
          <Box
            as="main"
            className="app-page-content"
            p={user ? { base: 3, sm: 4, md: 5, lg: 6 } : 0}
            minW={0}
            w="full"
            overflowX="clip"
          >
            {children}
          </Box>
        </Box>
      </Box>
    </ProtectedRoute>
  );
};

export default SimpleLayout;

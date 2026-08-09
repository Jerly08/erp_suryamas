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
  const [isSidebarCollapsed, setIsSidebarCollapsed] = React.useState(false);
  const [hasLoadedSidebarPreference, setHasLoadedSidebarPreference] = React.useState(false);
  const normalizedAllowed = normalizeRoles(allowedRoles as string[]);
  const isMobile = useBreakpointValue({ base: true, lg: false }, { fallback: 'lg' });
  
  // Dynamic background colors based on theme
  const bgColor = useColorModeValue('var(--bg-secondary)', 'var(--bg-primary)');

  React.useEffect(() => {
    const savedPreference = window.localStorage.getItem('avery.sidebar.collapsed');
    setIsSidebarCollapsed(savedPreference === 'true');
    setHasLoadedSidebarPreference(true);
  }, []);

  React.useEffect(() => {
    if (!hasLoadedSidebarPreference) return;

    window.localStorage.setItem('avery.sidebar.collapsed', String(isSidebarCollapsed));
  }, [hasLoadedSidebarPreference, isSidebarCollapsed]);

  const toggleSidebar = () => {
    setIsSidebarCollapsed((collapsed) => !collapsed);
  };

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
        {showSidebar && !isMobile && user && !isSidebarCollapsed && (
          <Sidebar 
            isOpen
            onClose={onClose} 
            display={{ base: 'none', lg: 'block' }}
            width={60}
            collapsed={isSidebarCollapsed}
            onToggleCollapse={toggleSidebar}
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
        <Box
          ml={{ base: 0, lg: showSidebar && !isSidebarCollapsed ? 60 : 0 }}
          minW={0}
          transition="margin-left 0.2s ease"
        >
          {/* Top navigation bar - always show if user is authenticated */}
          {user && (
            <SimpleNavbar 
              onMenuClick={showSidebar ? onOpen : undefined} 
              isMenuOpen={isOpen} 
              onSidebarToggle={showSidebar ? toggleSidebar : undefined}
              isSidebarCollapsed={isSidebarCollapsed}
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

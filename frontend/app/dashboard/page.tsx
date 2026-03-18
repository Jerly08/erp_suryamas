'use client';

import React, { useEffect, useMemo, useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useRouter } from 'next/navigation';
import {
    AdminDashboard,
    FinanceDashboard,
    InventoryManagerDashboard,
    DirectorDashboard,
    EmployeeDashboard
} from '@/components/dashboard';
import SimpleLayout from '@/components/layout/SimpleLayout';
import { useDashboardAnalytics } from '@/hooks/useDashboardAnalytics';
import {
  Flex,
  VStack,
  Spinner,
  Text,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  useToast,
} from '@chakra-ui/react';

export default function DashboardPage() {
  const { user, token } = useAuth();
  const router = useRouter();
  const [redirecting, setRedirecting] = useState(false);

  // Use shared analytics hook (real data only, no dummy)
  const { analytics, loading: analyticsLoading, error } = useDashboardAnalytics(user, token);

  // Redirect to login if not authenticated
  useEffect(() => {
    if (!user || !token) {
      router.push('/login');
    }
  }, [user, token, router]);

  // Handle unauthorized role redirect
  useEffect(() => {
    if (user && !['admin', 'finance', 'finance_manager', 'inventory_manager', 'director', 'employee'].includes(user.role)) {
      setRedirecting(true);
      router.push('/unauthorized');
    }
  }, [user, router]);

  const toast = useToast();

  const roleNeedsAnalytics = useMemo(() => {
    const role = user?.role;
    return role === 'admin' || role === 'director' || role === 'finance' || role === 'finance_manager';
  }, [user]);

  const isLoading = redirecting || (roleNeedsAnalytics ? analyticsLoading : false);

  const renderDashboardByRole = () => {
    if (isLoading) {
      return (
        <Flex justify="center" align="center" minH="60vh">
          <VStack spacing={4}>
            <Spinner size="xl" color="brand.500" thickness="4px" />
            <Text>{redirecting ? 'Mengalihkan...' : 'Memuat dasbor...'}</Text>
          </VStack>
        </Flex>
      );
    }

    if (error && roleNeedsAnalytics) {
      return (
        <Alert status="error" borderRadius="md">
          <AlertIcon />
          <AlertTitle mr={2}>Error!</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      );
    }

    switch (user?.role) {
      case 'admin':
        return <AdminDashboard analytics={analytics} />;
      case 'finance':
      case 'finance_manager':
        return <FinanceDashboard analytics={analytics} />;
      case 'inventory_manager':
        return <InventoryManagerDashboard />;
      case 'director':
        return <DirectorDashboard analytics={analytics} />;
      case 'employee':
        return <EmployeeDashboard />;
      default:
        return (
          <Flex justify="center" align="center" minH="60vh">
            <VStack spacing={4}>
              <Spinner size="xl" color="brand.500" thickness="4px" />
              <Text>Mengalihkan ke halaman yang sesuai...</Text>
            </VStack>
          </Flex>
        );
    }
  };

  return (
    <SimpleLayout allowedRoles={['admin', 'finance', 'finance_manager', 'director', 'inventory_manager', 'employee']}>
      {renderDashboardByRole()}
    </SimpleLayout>
  );
}

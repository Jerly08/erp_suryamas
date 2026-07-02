"use client";

import React, { useEffect, useMemo, useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { useRouter } from "next/navigation";
import { isFinanceManagerRole, normalizeRole } from "@/utils/roles";
import {
  AdminDashboard,
  COAApprovalQueueCard,
  FinanceDashboard,
  FinanceStaffDashboard,
  InventoryManagerDashboard,
  DirectorDashboard,
  EmployeeDashboard,
} from "@/components/dashboard";
import SimpleLayout from "@/components/layout/SimpleLayout";
import { useDashboardAnalytics } from "@/hooks/useDashboardAnalytics";
import {
  Flex,
  VStack,
  Spinner,
  Text,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Button,
} from "@chakra-ui/react";

const AccountingDashboard = () => (
  <Flex justify="center" align="center" minH="16vh">
    <VStack spacing={2}>
      <Text fontSize="xl" fontWeight="bold">
        Dashboard Accounting
      </Text>
      <Text color="gray.500" textAlign="center">
        Fokus dashboard ini adalah monitoring purchase yang menunggu input
        GL/COA dari tim Accounting.
      </Text>
    </VStack>
  </Flex>
);

export default function DashboardPage() {
  const { user } = useAuth();
  const router = useRouter();
  const [redirecting, setRedirecting] = useState(false);

  const roleNeedsAnalytics = useMemo(() => {
    const role = normalizeRole(user?.role);
    return (
      role === "admin" || role === "director" || isFinanceManagerRole(role)
    );
  }, [user]);

  // Use shared analytics hook only for roles that need manager-level summary charts
  const {
    analytics,
    loading: analyticsLoading,
    error,
  } = useDashboardAnalytics(user, roleNeedsAnalytics);

  // Redirect to login if not authenticated
  useEffect(() => {
    if (!user) {
      router.push("/login");
    }
  }, [user, router]);

  // Handle unauthorized role redirect
  useEffect(() => {
    const role = normalizeRole(user?.role);
    if (
      user &&
      ![
        "admin",
        "finance",
        "finance_manager",
        "finance_staff",
        "accounting",
        "hr",
        "inventory_manager",
        "director",
        "employee",
      ].includes(role)
    ) {
      setRedirecting(true);
      router.push("/unauthorized");
    }
  }, [user, router]);

  const isLoading =
    redirecting || (roleNeedsAnalytics ? analyticsLoading : false);

  const role = normalizeRole(user?.role);
  const shouldShowCOAApprovalQueue =
    !isLoading &&
    !error &&
    ["admin", "finance", "finance_manager", "accounting"].includes(role);

  const renderDashboardByRole = () => {
    if (isLoading) {
      return (
        <Flex justify="center" align="center" minH="60vh">
          <VStack spacing={4}>
            <Spinner size="xl" color="brand.500" thickness="4px" />
            <Text>{redirecting ? "Mengalihkan..." : "Memuat dasbor..."}</Text>
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

    switch (role) {
      case "admin":
        return <AdminDashboard analytics={analytics} />;
      case "finance":
      case "finance_manager":
        return <FinanceDashboard analytics={analytics} />;
      case "finance_staff":
        return <FinanceStaffDashboard />;
      case "accounting":
        return <AccountingDashboard />;
      case "hr":
        return (
          <Flex justify="center" align="center" minH="60vh">
            <VStack spacing={4}>
              <Text fontSize="xl" fontWeight="bold">Dashboard HR</Text>
              <Text color="gray.500" textAlign="center">
                Kelola input payroll dan kirimkan ke Finance Manager untuk approval.
              </Text>
              <Button colorScheme="blue" onClick={() => router.push("/payroll")}>
                Buka Modul Payroll
              </Button>
            </VStack>
          </Flex>
        );
      case "inventory_manager":
        return <InventoryManagerDashboard />;
      case "director":
        return <DirectorDashboard analytics={analytics} />;
      case "employee":
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
    <SimpleLayout
      allowedRoles={[
        "admin",
        "finance",
        "finance_manager",
        "finance_staff",
        "accounting",
        "hr",
        "director",
        "inventory_manager",
        "employee",
      ]}
    >
      {shouldShowCOAApprovalQueue ? (
        <VStack spacing={6} align="stretch">
          <COAApprovalQueueCard />
          {renderDashboardByRole()}
        </VStack>
      ) : (
        renderDashboardByRole()
      )}
    </SimpleLayout>
  );
}

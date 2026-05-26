'use client';

import React, { useState, useEffect } from 'react';
import {
  Box,
  VStack,
  HStack,
  Heading,
  Text,
  Card,
  CardBody,
  CardHeader,
  Button,
  Badge,
  Grid,
  GridItem,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  TableContainer,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  useDisclosure,
  Textarea,
  FormControl,
  FormLabel,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  IconButton,
  Spinner,
  Stat,
  StatLabel,
  StatNumber,
  useToast,
  Tooltip,
  Divider,
  Flex,
  Center,
} from '@chakra-ui/react';
import {
  FiCheckCircle,
  FiXCircle,
  FiEye,
  FiClock,
  FiRefreshCw,
  FiTrendingUp,
  FiList,
} from 'react-icons/fi';
import { useAuth } from '@/contexts/AuthContext';
import approvalService, { Purchase, ApprovalStats, ApprovalHistory } from '@/services/approvalService';
import WorkflowVisualization from '@/components/approval/WorkflowVisualization';
import { normalizeRole, humanizeRole, workflowRoleMatchesUserRole } from '@/utils/roles';
import { getUserDisplayName } from '@/utils/userDisplay';

interface PurchaseApprovalDashboardProps {}

const PurchaseApprovalDashboard: React.FC<PurchaseApprovalDashboardProps> = () => {
  const { user, token } = useAuth();
  const toast = useToast();
  const { 
    isOpen: isApprovalOpen, 
    onOpen: onApprovalOpen, 
    onClose: onApprovalClose 
  } = useDisclosure();
  const { 
    isOpen: isHistoryOpen, 
    onOpen: onHistoryOpen, 
    onClose: onHistoryClose 
  } = useDisclosure();
  const {
    isOpen: isWorkflowOpen,
    onOpen: onWorkflowOpen,
    onClose: onWorkflowClose,
  } = useDisclosure();

  const [purchases, setPurchases] = useState<Purchase[]>([]);
  const [loading, setLoading] = useState(true);
  const [stats, setStats] = useState<ApprovalStats>({
    pending_approvals: 0,
    approved_this_month: 0,
    rejected_this_month: 0,
    total_amount_pending: 0,
  });
  const [selectedPurchase, setSelectedPurchase] = useState<Purchase | null>(null);
  const [approvalType, setApprovalType] = useState<'approve' | 'reject'>('approve');
  const [comments, setComments] = useState('');
  const [approvalHistory, setApprovalHistory] = useState<ApprovalHistory[]>([]);
  const [processing, setProcessing] = useState(false);
  const [requiresDirector, setRequiresDirector] = useState(false);

  useEffect(() => {
    if (token) {
      fetchPurchasesForApproval();
      fetchApprovalStats();
    }
  }, [token]);

  const fetchPurchasesForApproval = async () => {
    try {
      setLoading(true);
      const response = await approvalService.getPurchasesForApproval();
      setPurchases(response.purchases || []);
    } catch (error: any) {
      console.error('Failed to fetch purchases for approval:', error);
      toast({
        title: 'Error',
        description: 'Failed to fetch purchases for approval',
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  const fetchApprovalStats = async () => {
    try {
      const statsData = await approvalService.getApprovalStats();
      setStats(statsData);
    } catch (error: any) {
      console.error('Failed to fetch approval stats:', error);
      toast({
        title: 'Warning',
        description: 'Failed to fetch approval statistics',
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
    }
  };

  const handleApprove = async () => {
    if (!selectedPurchase) return;
    
    setProcessing(true);
    try {
      // Check if user is Finance and wants to escalate to Director
      const userRole = normalizeRole(user?.role as string);
      const isFinance = userRole === 'finance';
      // Remove amount restriction - Finance can escalate any amount to Director
      const needsEscalation = isFinance && requiresDirector;
      
      const response = await approvalService.approvePurchase(selectedPurchase.id, {
        comments: comments || undefined,
        escalate_to_director: needsEscalation
      });
      
      // Check if it was escalated
      if (response?.escalated) {
        toast({
          title: 'Escalated to Director',
          description: response.message || 'Purchase has been escalated to Director for final approval',
          status: 'info',
          duration: 5000,
          isClosable: true,
        });
      } else {
        toast({
          title: 'Success',
          description: response?.message || 'Purchase approved successfully',
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      }
      
      onApprovalClose();
      setComments('');
      setSelectedPurchase(null);
      // Refresh data to show updated status
      await fetchPurchasesForApproval();
      await fetchApprovalStats();
    } catch (error: any) {
      console.error('Failed to approve purchase:', error);
      toast({
        title: 'Error',
        description: error.response?.data?.error || error.response?.data?.message || 'Failed to approve purchase',
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setProcessing(false);
    }
  };

  const handleReject = async () => {
    if (!selectedPurchase || !comments.trim()) {
      toast({
        title: 'Warning',
        description: 'Please provide a reason for rejection',
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }
    
    setProcessing(true);
    try {
      await approvalService.rejectPurchase(selectedPurchase.id, {
        comments: comments
      });
      toast({
        title: 'Success',
        description: 'Purchase rejected successfully',
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
      onApprovalClose();
      setComments('');
      setSelectedPurchase(null);
      await fetchPurchasesForApproval();
      await fetchApprovalStats();
    } catch (error: any) {
      console.error('Failed to reject purchase:', error);
      toast({
        title: 'Error',
        description: error.response?.data?.message || 'Failed to reject purchase',
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setProcessing(false);
    }
  };

  const handleViewHistory = async (purchase: Purchase) => {
    try {
      const response = await approvalService.getApprovalHistory(purchase.id);
      setApprovalHistory(response.approval_history || []);
      setSelectedPurchase(purchase);
      onHistoryOpen();
    } catch (error: any) {
      console.error('Failed to fetch approval history:', error);
      toast({
        title: 'Error',
        description: 'Failed to fetch approval history',
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    }
  };

  const openApprovalDialog = (purchase: Purchase, type: 'approve' | 'reject') => {
    setSelectedPurchase(purchase);
    setApprovalType(type);
    setComments('');
    onApprovalOpen();
  };

  const openWorkflowDialog = (purchase: Purchase) => {
    setSelectedPurchase(purchase);
    onWorkflowOpen();
  };

  const getStatusBadge = (status: string) => {
    switch (status.toLowerCase()) {
      case 'pending_approval':
      case 'pending':
        return <Badge colorScheme="yellow" variant="subtle">Pending</Badge>;
      case 'approved':
        return <Badge colorScheme="green" variant="subtle">Approved</Badge>;
      case 'rejected':
        return <Badge colorScheme="red" variant="subtle">Rejected</Badge>;
      default:
        return <Badge colorScheme="gray" variant="subtle">{status}</Badge>;
    }
  };

const getPriorityColor = (amount: number) => {
    if (amount > 50000000) return 'red.500'; // > 50M IDR
    if (amount > 25000000) return 'orange.500'; // > 25M IDR
    return 'blue.500'; // <= 25M IDR
  };

  // Determine active step's approver role from approval_steps
  const getActiveStepRole = (purchase: Purchase): string | null => {
    // approvalService types show approval_steps: ApprovalAction[]
    // Active step is where is_active = true and status is PENDING
    // Fallback: first step with status PENDING
    const steps = (purchase as any).approval_steps as any[] | undefined;
    if (!steps || steps.length === 0) return null;
    const active = steps.find((a: any) => a.is_active && a.status === 'PENDING');
    if (active?.step?.approver_role) return normalizeRole(active.step.approver_role);
    const pending = steps.find((a: any) => a.status === 'PENDING');
    if (pending?.step?.approver_role) return normalizeRole(pending.step.approver_role);
    return null;
  };

  // Get active step details for display
  const getActiveStepInfo = (purchase: Purchase): { role: string | null, humanRole: string, hasActiveStep: boolean, stepStatus: string } => {
    const steps = (purchase as any).approval_steps as any[] | undefined;
    if (!steps || steps.length === 0) {
      return { role: null, humanRole: 'Tidak ada workflow', hasActiveStep: false, stepStatus: 'NO_WORKFLOW' };
    }
    
    // First check for active steps
    const active = steps.find((a: any) => a.is_active && a.status === 'PENDING');
    if (active?.step?.approver_role) {
      const normalizedRole = normalizeRole(active.step.approver_role);
      return {
        role: normalizedRole,
        humanRole: `Menunggu ${humanizeRole(normalizedRole)}`,
        hasActiveStep: true,
        stepStatus: 'ACTIVE_PENDING'
      };
    }
    
    // Check for approved steps to show progression
    const approvedSteps = steps.filter((a: any) => a.status === 'APPROVED');
    const pendingSteps = steps.filter((a: any) => a.status === 'PENDING');
    
    if (pendingSteps.length > 0) {
      const nextPending = pendingSteps[0]; // Should be next in line
      const normalizedRole = normalizeRole(nextPending.step.approver_role);
      return {
        role: normalizedRole,
        humanRole: `Menunggu ${humanizeRole(normalizedRole)}`,
        hasActiveStep: false,
        stepStatus: 'NEXT_PENDING'
      };
    }
    
    // All steps approved
    if (approvedSteps.length === steps.length) {
      return { role: null, humanRole: 'Semua langkah disetujui', hasActiveStep: false, stepStatus: 'ALL_APPROVED' };
    }
    
    return { role: null, humanRole: 'Status tidak diketahui', hasActiveStep: false, stepStatus: 'UNKNOWN' };
  };

  // Check if current user can approve: must match active step approver_role.
  const canUserApprove = (purchase: Purchase, userRole?: string) => {
    const roleNorm = normalizeRole(userRole || '');
    if (!roleNorm) return false;
    
    const stepInfo = getActiveStepInfo(purchase);
    // Only allow approval if there's an active step and user role matches
    return stepInfo.hasActiveStep && workflowRoleMatchesUserRole(stepInfo.role, roleNorm);
  };

  // Get tooltip text for why user cannot approve
  const getApprovalTooltipText = (purchase: Purchase, userRole?: string): string => {
    const roleNorm = normalizeRole(userRole || '');
    const stepInfo = getActiveStepInfo(purchase);
    
    if (!stepInfo.hasActiveStep) {
      if (stepInfo.role) {
        return `Menunggu persetujuan ${stepInfo.humanRole}`;
      }
      return 'Tidak ada step approval yang aktif';
    }
    
    if (roleNorm === 'admin') {
      return 'Admin override: Approve purchase';
    }
    
    if (stepInfo.role !== roleNorm) {
      return `Menunggu persetujuan ${stepInfo.humanRole}`;
    }
    
    return 'Approve purchase';
  };

  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
    }).format(amount);
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('id-ID', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  // SLA countdown ticker (updates every minute)
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 60000);
    return () => clearInterval(id);
  }, []);

  // Compute SLA info for active step based on step.time_limit and activated_at/date
  const getSlaInfo = (purchase: Purchase): { label: string | null; color: string } => {
    const steps = (purchase as any).approval_steps as any[] | undefined;
    if (!steps || steps.length === 0) return { label: null, color: 'gray.500' };
    const active = steps.find((a: any) => a.is_active && a.status === 'PENDING');
    if (!active) return { label: null, color: 'gray.500' };

    const limitHours: number | undefined = active?.step?.time_limit;
    if (!limitHours || limitHours <= 0) return { label: null, color: 'gray.500' };

    const startAtStr: string | undefined = active.activated_at || purchase.date;
    if (!startAtStr) return { label: null, color: 'gray.500' };
    const startAt = new Date(startAtStr).getTime();
    const dueAt = startAt + limitHours * 60 * 60 * 1000;
    const diffMs = dueAt - now;

    const absMs = Math.abs(diffMs);
    const hours = Math.floor(absMs / (1000 * 60 * 60));
    const minutes = Math.floor((absMs % (1000 * 60 * 60)) / (1000 * 60));

    if (diffMs >= 0) {
      const label = `Due in ${hours} jam ${minutes} menit`;
      let color = 'green.500';
      if (hours < 24) color = 'orange.500';
      return { label, color };
    } else {
      const label = `Overdue by ${hours} jam ${minutes} menit`;
      return { label, color: 'red.500' };
    }
  };

  if (loading) {
    return (
      <Center minHeight="400px">
        <VStack spacing={4}>
          <Spinner size="xl" />
          <Text>Loading approval dashboard...</Text>
        </VStack>
      </Center>
    );
  }

  // Normalize current user role once
  const meRole = normalizeRole((user?.role as unknown as string) || '');

  // Employee: show empty-state if accessing this dashboard
  if (meRole === 'employee') {
    return (
      <VStack spacing={6} align="stretch">
        <Box>
          <Heading size="lg" mb={2}>Purchase Approval Dashboard</Heading>
          <Text color="gray.600">Menu ini khusus untuk persetujuan oleh peran Finance/Director.</Text>
        </Box>
        <Card>
          <CardBody>
            <Alert status="info">
              <AlertIcon />
              <AlertTitle mr={2}>Tidak ada item untuk Anda</AlertTitle>
              <AlertDescription>Persetujuan dilakukan oleh Finance/Director. Anda dapat membuat Purchase dan Submit for Approval dari menu Purchases.</AlertDescription>
            </Alert>
          </CardBody>
        </Card>
      </VStack>
    );
  }

  // Client-side filter based on active step approver_role to avoid confusion
  const filteredPurchases = purchases.filter((p) => {
    const nextRole = getActiveStepRole(p);
    if (!nextRole) return true; // if unknown, show it
    return meRole === 'admin' || workflowRoleMatchesUserRole(nextRole, meRole);
  });

  // Count pending items for other roles (for informative empty state)
  const otherPendingCount = purchases.filter((p) => {
    if (meRole === 'admin') return false; // admin can see/act on all
    const status = (p.approval_status || '').toLowerCase();
    const isPending = status === 'pending' || status === 'pending_approval';
    const stepInfo = getActiveStepInfo(p);
    return isPending && stepInfo.hasActiveStep && !workflowRoleMatchesUserRole(stepInfo.role, meRole);
  }).length;

  // Derive next approver label for header (from first item, if any)
  const nextApproverRole = (() => {
    const first = purchases[0];
    const r = first ? getActiveStepRole(first) : null;
    return r ? humanizeRole(r) : 'Unknown';
  })();

  // Get summary of active approvers needed
  const getActiveApproversSummary = () => {
    const approverCounts = new Map<string, number>();
    filteredPurchases.forEach(purchase => {
      const stepInfo = getActiveStepInfo(purchase);
      if (stepInfo.hasActiveStep && stepInfo.humanRole) {
        const current = approverCounts.get(stepInfo.humanRole) || 0;
        approverCounts.set(stepInfo.humanRole, current + 1);
      }
    });
    
    const entries = Array.from(approverCounts.entries());
    if (entries.length === 0) return null;
    
    return entries.map(([role, count]) => `${role} (${count})`).join(', ');
  };

  // Sum total amount from filtered purchases (for role-specific landing card)
  const filteredTotalAmount = filteredPurchases.reduce((acc, p) => acc + (p.total_amount || 0), 0);

  return (
    <VStack spacing={6} align="stretch">
      {/* Header */}
      <Box>
        <Heading size="lg" mb={2}>Purchase Approval Dashboard</Heading>
        <HStack spacing={3} wrap="wrap">
          <Badge colorScheme="purple" variant="subtle">Next approver: {nextApproverRole}</Badge>
          <Badge colorScheme="blue" variant="subtle">Your role: {humanizeRole(user?.role as unknown as string)}</Badge>
          {getActiveApproversSummary() && (
            <Badge colorScheme="orange" variant="subtle">
              Active approvers needed: {getActiveApproversSummary()}
            </Badge>
          )}
        </HStack>
        <Text color="gray.600" mt={2}>
          Review and approve pending purchase requests based on your role.
        </Text>
      </Box>

      {/* Role Information Alert - Check if user has any pending approvals */}
      {filteredPurchases.length > 0 && !filteredPurchases.some(p => canUserApprove(p, user?.role as unknown as string)) && (
        <Alert status="warning">
          <AlertIcon />
          <AlertTitle>Limited Access!</AlertTitle>
          <AlertDescription>
            Your role ({humanizeRole(user?.role as unknown as string) || 'Unknown'}) cannot approve any of the current pending purchases. 
            Waiting for appropriate approvers.
          </AlertDescription>
        </Alert>
      )}

      {/* Role-specific Landing Card for Finance/Director */}
      {(meRole === 'finance' || meRole === 'director') && (
        <Card>
          <CardBody>
            <HStack justify="space-between">
              <VStack align="start" spacing={0}>
                <Text fontWeight="bold">Persetujuan menunggu Anda</Text>
                <Text color="gray.600" fontSize="sm">{filteredPurchases.length} item · {formatCurrency(filteredTotalAmount)}</Text>
              </VStack>
              <Box color="orange.400">
                <FiClock size={28} />
              </Box>
            </HStack>
          </CardBody>
        </Card>
      )}

      {/* Stats Cards */}
      <Grid templateColumns="repeat(auto-fit, minmax(250px, 1fr))" gap={4}>
        <GridItem>
          <Card>
            <CardBody>
              <Stat>
                <HStack>
                  <Box color="orange.500">
                    <FiClock size={24} />
                  </Box>
                  <VStack align="start" spacing={0}>
                    <StatLabel>Pending Approvals</StatLabel>
                    <StatNumber color="orange.500">{stats.pending_approvals}</StatNumber>
                  </VStack>
                </HStack>
              </Stat>
            </CardBody>
          </Card>
        </GridItem>

        <GridItem>
          <Card>
            <CardBody>
              <Stat>
                <HStack>
                  <Box color="blue.500">
                    <FiTrendingUp size={24} />
                  </Box>
                  <VStack align="start" spacing={0}>
                    <StatLabel>Total Pending Amount</StatLabel>
                    <StatNumber fontSize="md" color="blue.500">
                      {formatCurrency(stats.total_amount_pending)}
                    </StatNumber>
                  </VStack>
                </HStack>
              </Stat>
            </CardBody>
          </Card>
        </GridItem>

        <GridItem>
          <Card>
            <CardBody>
              <Stat>
                <HStack>
                  <Box color="green.500">
                    <FiCheckCircle size={24} />
                  </Box>
                  <VStack align="start" spacing={0}>
                    <StatLabel>Approved This Month</StatLabel>
                    <StatNumber color="green.500">{stats.approved_this_month}</StatNumber>
                  </VStack>
                </HStack>
              </Stat>
            </CardBody>
          </Card>
        </GridItem>

        <GridItem>
          <Card>
            <CardBody>
              <Stat>
                <HStack>
                  <Box color="red.500">
                    <FiXCircle size={24} />
                  </Box>
                  <VStack align="start" spacing={0}>
                    <StatLabel>Rejected This Month</StatLabel>
                    <StatNumber color="red.500">{stats.rejected_this_month}</StatNumber>
                  </VStack>
                </HStack>
              </Stat>
            </CardBody>
          </Card>
        </GridItem>
      </Grid>

      {/* Purchases Table */}
      <Card>
        <CardHeader>
          <Flex justify="space-between" align="center">
            <Heading size="md">
              Purchases Requiring Approval ({filteredPurchases.length})
            </Heading>
            <Tooltip label="Refresh">
              <IconButton
                aria-label="Refresh"
                icon={<FiRefreshCw />}
                onClick={fetchPurchasesForApproval}
                variant="outline"
                size="sm"
              />
            </Tooltip>
          </Flex>
        </CardHeader>
        <CardBody p={0}>
          {filteredPurchases.length === 0 ? (
            <Box p={6}>
              <VStack align="stretch" spacing={3}>
                <Alert status="info">
                  <AlertIcon />
                  <AlertDescription>
                    Tidak ada persetujuan yang menunggu peran Anda ({humanizeRole(user?.role as unknown as string)}).
                  </AlertDescription>
                </Alert>
                {otherPendingCount > 0 && (
                  <Alert status="warning" variant="subtle">
                    <AlertIcon />
                    <AlertDescription>
                      Terdapat {otherPendingCount} item yang sedang menunggu persetujuan peran lain. Baris-baris tersebut disembunyikan dari tampilan Anda.
                    </AlertDescription>
                  </Alert>
                )}
              </VStack>
            </Box>
          ) : (
            <VStack align="stretch" spacing={0}>
              {otherPendingCount > 0 && (
                <Box p={4}>
                  <Alert status="warning" variant="subtle">
                    <AlertIcon />
                    <AlertDescription>
                      Info: {otherPendingCount} item lain sedang menunggu peran berbeda. Anda hanya melihat item yang relevan untuk peran Anda.
                    </AlertDescription>
                  </Alert>
                </Box>
              )}
              <TableContainer>
                <Table variant="simple">
                <Thead>
                  <Tr>
                    <Th>Purchase Code</Th>
                    <Th>Vendor</Th>
                    <Th>Amount</Th>
                    <Th>Date</Th>
                    <Th>Status</Th>
                    <Th>Active Approver</Th>
                    <Th>Requester</Th>
                    <Th>Actions</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {filteredPurchases.map((purchase) => {
                    const stepInfo = getActiveStepInfo(purchase);
                    const canApprove = canUserApprove(purchase, user?.role as unknown as string);
                    const approvalTooltip = getApprovalTooltipText(purchase, user?.role as unknown as string);
                    
                    return (
                    <Tr key={purchase.id}>
                      <Td>
                        <Text fontWeight="semibold">{purchase.code}</Text>
                      </Td>
                      <Td>{purchase.vendor?.name || 'Unknown Vendor'}</Td>
                      <Td>
                        <Text 
                          fontWeight="bold"
                          color={getPriorityColor(purchase.total_amount)}
                        >
                          {formatCurrency(purchase.total_amount)}
                        </Text>
                      </Td>
                      <Td>
                        {new Date(purchase.date).toLocaleDateString('id-ID')}
                      </Td>
                      <Td>{getStatusBadge(purchase.approval_status)}</Td>
                      <Td>
                        <VStack align="start" spacing={1}>
                          <Badge 
                            colorScheme={stepInfo.hasActiveStep ? 'green' : 'gray'} 
                            variant="subtle"
                            size="sm"
                            w="fit-content"
                          >
                            {stepInfo.humanRole}
                          </Badge>
                          {stepInfo.hasActiveStep && (() => { const sla = getSlaInfo(purchase); return sla.label ? (
                            <Text fontSize="xs" color={sla.color}>{sla.label}</Text>
                          ) : null; })()}
                        </VStack>
                      </Td>
                      <Td>
                        {getUserDisplayName(purchase.user, {
                          fallback: 'Unknown User',
                          allowRoleFallback: false,
                        })}
                      </Td>
                      <Td>
                        <HStack spacing={1}>
                          <Tooltip label="Lihat alur">
                            <Button
                              variant="link"
                              size="xs"
                              onClick={() => openWorkflowDialog(purchase)}
                            >
                              Lihat alur
                            </Button>
                          </Tooltip>
                          <Tooltip label="View Details">
                            <IconButton
                              aria-label="View"
                              icon={<FiEye />}
                              size="sm"
                              variant="ghost"
                            />
                          </Tooltip>
                          
                          {/* Approve Button */}
                          <Tooltip label={approvalTooltip}>
                            <IconButton
                              aria-label={meRole === 'admin' ? 'Admin override: Approve' : 'Approve'}
                              icon={<FiCheckCircle />}
                              size="sm"
                              variant="ghost"
                              colorScheme="green"
                              isDisabled={!canApprove && meRole !== 'admin'}
                              onClick={() => (canApprove || meRole === 'admin') && openApprovalDialog(purchase, 'approve')}
                            />
                          </Tooltip>
                          
                          {/* Reject Button */}
                          <Tooltip label={canApprove || meRole === 'admin' ? (meRole === 'admin' ? 'Admin override: Reject purchase' : 'Reject purchase') : approvalTooltip}>
                            <IconButton
                              aria-label={meRole === 'admin' ? 'Admin override: Reject' : 'Reject'}
                              icon={<FiXCircle />}
                              size="sm"
                              variant="ghost"
                              colorScheme="red"
                              isDisabled={!canApprove && meRole !== 'admin'}
                              onClick={() => (canApprove || meRole === 'admin') && openApprovalDialog(purchase, 'reject')}
                            />
                          </Tooltip>
                          {meRole === 'admin' && (
                            <Badge colorScheme="purple" variant="subtle">Admin override</Badge>
                          )}
                          
                          <Tooltip label="View History">
                            <IconButton
                              aria-label="History"
                              icon={<FiList />}
                              size="sm"
                              variant="ghost"
                              onClick={() => handleViewHistory(purchase)}
                            />
                          </Tooltip>
                        </HStack>
                      </Td>
                    </Tr>
                    );
                  })}
                </Tbody>
              </Table>
            </TableContainer>
            </VStack>
          )}
        </CardBody>
      </Card>

      {/* Approval Modal */}
      <Modal isOpen={isApprovalOpen} onClose={onApprovalClose} size="lg">
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>
            {approvalType === 'approve' ? 'Approve Purchase' : 'Reject Purchase'}
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            {selectedPurchase && (
              <VStack spacing={4} align="stretch">
                <Box>
                  <Text fontWeight="bold" fontSize="lg">{selectedPurchase.code}</Text>
                  <Text color="gray.600">Vendor: {selectedPurchase.vendor?.name}</Text>
                  <Text color="gray.600">
                    Amount: {formatCurrency(selectedPurchase.total_amount)}
                  </Text>
                  <Text color="gray.600">
                    Requested by: {getUserDisplayName(selectedPurchase.user, {
                      fallback: 'Unknown User',
                      allowRoleFallback: false,
                    })}
                  </Text>
                </Box>
                <Divider />
                <FormControl>
                  <FormLabel>
                    {approvalType === 'approve' ? 'Comments (Optional)' : 'Reason for Rejection*'}
                  </FormLabel>
                  <Textarea
                    value={comments}
                    onChange={(e) => setComments(e.target.value)}
                    placeholder={
                      approvalType === 'approve' 
                        ? 'Optional comments about the approval'
                        : 'Please provide a clear reason for rejection'
                    }
                    rows={4}
                  />
                </FormControl>
                {/* Show checkbox for Finance role to escalate to Director */}
                {approvalType === 'approve' && normalizeRole(user?.role as string) === 'finance' && (
                  <FormControl>
                    <HStack>
                      <input
                        type="checkbox"
                        id="escalateToDirector"
                        checked={requiresDirector}
                        onChange={(e) => setRequiresDirector(e.target.checked)}
                      />
                      <FormLabel htmlFor="escalateToDirector" mb={0} cursor="pointer">
                        Eskalasi ke Director untuk persetujuan tambahan
                      </FormLabel>
                    </HStack>
                  </FormControl>
                )}
              </VStack>
            )}
          </ModalBody>
          <ModalFooter>
            <Button variant="ghost" mr={3} onClick={onApprovalClose}>
              Cancel
            </Button>
            <Button
              colorScheme={approvalType === 'approve' ? 'green' : 'red'}
              onClick={approvalType === 'approve' ? handleApprove : handleReject}
              isLoading={processing}
              loadingText="Processing..."
            >
              {approvalType === 'approve' ? 'Approve' : 'Reject'}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* Workflow Modal */}
      <Modal isOpen={isWorkflowOpen} onClose={onWorkflowClose} size="4xl">
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>
            Alur Persetujuan - {selectedPurchase?.code}
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            {selectedPurchase ? (
              (() => {
                const actionsRaw = ((selectedPurchase as any).approval_steps || []) as any[];
                const actions = actionsRaw.map((a: any) => ({
                  id: a.id,
                  step_id: a.step_id || a.step?.id,
                  status: a.status,
                  approver_id: a.approver_id,
                  action_date: a.action_date,
                  is_active: a.is_active,
                  comments: a.comments,
                  approver: a.approver ? { first_name: a.approver.first_name, last_name: a.approver.last_name } : undefined,
                }));
                const stepMap = new Map<number, any>();
                actionsRaw.forEach((a: any) => {
                  const s = a.step || {};
                  const sid = a.step_id || s.id;
                  if (!sid) return;
                  if (!stepMap.has(sid)) {
                    stepMap.set(sid, {
                      id: sid,
                      step_order: s.step_order ?? 0,
                      step_name: s.step_name || `Step ${sid}`,
                      approver_role: s.approver_role || 'UNKNOWN',
                      is_optional: !!s.is_optional,
                      is_parallel: !!s.is_parallel,
                      time_limit: s.time_limit,
                    });
                  }
                });
                const steps = Array.from(stepMap.values());
                return (
                  <WorkflowVisualization
                    steps={steps}
                    actions={actions}
                    currentStatus={selectedPurchase.approval_status}
                  />
                );
              })()
            ) : (
              <Text>Tidak ada data workflow.</Text>
            )}
          </ModalBody>
          <ModalFooter>
            <Button onClick={onWorkflowClose}>Tutup</Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* History Modal */}
      <Modal isOpen={isHistoryOpen} onClose={onHistoryClose} size="xl">
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>Approval History - {selectedPurchase?.code}</ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            {approvalHistory.length === 0 ? (
              <Alert status="info">
                <AlertIcon />
                <AlertDescription>No approval history available</AlertDescription>
              </Alert>
            ) : (
              <TableContainer>
                <Table size="sm">
                  <Thead>
                    <Tr>
                      <Th>Date</Th>
                      <Th>Action</Th>
                      <Th>User</Th>
                      <Th>Comments</Th>
                    </Tr>
                  </Thead>
                  <Tbody>
                    {approvalHistory.map((entry, index) => (
                      <Tr key={index}>
                        <Td>{formatDate(entry.created_at)}</Td>
                        <Td>{getStatusBadge(entry.action)}</Td>
                        <Td>
                          {getUserDisplayName(entry.user, {
                            fallback: 'Unknown User',
                            allowRoleFallback: false,
                          })}
                        </Td>
                        <Td>{entry.comments || '-'}</Td>
                      </Tr>
                    ))}
                  </Tbody>
                </Table>
              </TableContainer>
            )}
          </ModalBody>
          <ModalFooter>
            <Button onClick={onHistoryClose}>Close</Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </VStack>
  );
};

export default PurchaseApprovalDashboard;

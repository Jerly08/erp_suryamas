'use client';
import React, { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import api from '@/services/api';
import { API_ENDPOINTS } from '@/config/api';
import { useTranslation } from '@/hooks/useTranslation';
import {
  Box,
  Heading,
  Text,
  Card,
  CardHeader,
  CardBody,
  Button,
  HStack,
  Icon,
  List,
  ListItem,
  ListIcon,
  Badge,
  Flex,
  Spinner
} from '@chakra-ui/react';
import {
  FiUser,
  FiPlus,
  FiFileText,
  FiActivity,
  FiBell
} from 'react-icons/fi';

interface DashboardSummary {
  statistics?: Record<string, any>;
  recent_activities?: Array<{
    id: number;
    action: string;
    table_name?: string;
    record_id?: number;
    user_id?: number;
    created_at?: string;
  }>;
  unread_notifications?: number;
  min_stock_alerts_count?: number;
}

interface PendingApproval {
  id: number;
  request_code: string;
  request_title: string;
  request_type: string;
  total_amount: number;
  priority: string;
  requester_name: string;
  created_at: string;
  step_name: string;
  time_limit: number;
  days_pending?: number;
  urgency_level?: string;
}

interface SubmittedRequest {
  id: number;
  request_code: string;
  request_title: string;
  request_type: string;
  total_amount: number;
  status: string;
  priority: string;
  created_at: string;
  completed_at?: string;
  current_step_name: string;
}

interface RecentNotification {
  id: number;
  type: string;
  title: string;
  message: string;
  priority: string;
  is_read: boolean;
  created_at: string;
  data?: string;
}

interface QuickStats {
  pending_approvals_count: number;
  submitted_requests_count: number;
  pending_requests_count: number;
  unread_notifications_count: number;
  purchase_requests_count?: number;
  approved_purchase_requests_count?: number;
  rejected_purchase_requests_count?: number;
}

interface EmployeeDashboardData {
  pending_approvals?: PendingApproval[];
  submitted_requests?: SubmittedRequest[];
  recent_notifications?: RecentNotification[];
  quick_stats?: QuickStats;
}

export const EmployeeDashboard = () => {
  const router = useRouter();
  const { t } = useTranslation();
  const [employeeData, setEmployeeData] = useState<EmployeeDashboardData | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchEmployeeDashboard = async () => {
      try {
        const res = await api.get(API_ENDPOINTS.DASHBOARD_EMPLOYEE);
        setEmployeeData(res.data?.data || res.data || {});
        setError(null);
      } catch (e: any) {
        console.error('Employee dashboard fetch error:', e);
        setError(e?.response?.data?.error || e?.message || t('messages.operationFailed'));
      } finally {
        setLoading(false);
      }
    };
    fetchEmployeeDashboard();
  }, []);

  const formatCurrency = (value: number) =>
    new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', minimumFractionDigits: 0 }).format(value || 0);

  return (
    <Box>
      <Heading as="h2" size="xl" mb={6} color="gray.800">
        {t('navigation.dashboard')} - {t('users.employee')}
      </Heading>

      {loading ? (
        <Flex justify="center" align="center" minH="120px">
          <Spinner size="lg" color="brand.500" thickness="4px" />
        </Flex>
      ) : (
        <>
          {error && (
            <Box bg="red.50" p={4} borderRadius="lg" borderLeft="4px solid" borderColor="red.500" mb={4}>
              <Text color="red.700">{error}</Text>
            </Box>
          )}

          {/* Quick Stats Summary */}
          {employeeData?.quick_stats && (
            <Flex gap={4} flexWrap="wrap" mt={2}>
              <Box bg="white" p={4} borderRadius="lg" boxShadow="sm" flex="1" minW="180px">
                <Heading as="h3" size="sm" mb={2} color="orange.600">{t('dashboard.pendingApprovals')}</Heading>
                <Text fontSize="2xl" fontWeight="bold" color="orange.600">
                  {employeeData.quick_stats.pending_approvals_count}
                </Text>
              </Box>
              <Box bg="white" p={4} borderRadius="lg" boxShadow="sm" flex="1" minW="180px">
                <Heading as="h3" size="sm" mb={2} color="blue.600">{t('purchases.purchaseOrder')}</Heading>
                <Text fontSize="2xl" fontWeight="bold" color="blue.600">
                  {employeeData.quick_stats.submitted_requests_count}
                </Text>
                {employeeData.quick_stats.pending_requests_count > 0 && (
                  <Text fontSize="sm" color="gray.500">
                    {employeeData.quick_stats.pending_requests_count} pending
                  </Text>
                )}
              </Box>
              <Box bg="white" p={4} borderRadius="lg" boxShadow="sm" flex="1" minW="180px">
                <Heading as="h3" size="sm" mb={2} color="red.600">
                  <Icon as={FiBell} mr={1} />{t('common.labels.notification') || 'Notifikasi'}
                </Heading>
                <Text fontSize="2xl" fontWeight="bold" color="red.600">
                  {employeeData.quick_stats.unread_notifications_count}
                </Text>
                <Text fontSize="sm" color="gray.500">belum dibaca</Text>
              </Box>
            </Flex>
          )}

          {/* Pending Approvals */}
          {employeeData?.pending_approvals && employeeData.pending_approvals.length > 0 && (
            <Card mt={6}>
              <CardHeader>
                <Heading size="md" display="flex" alignItems="center">
                  <Icon as={FiBell} mr={2} color="orange.500" />
                  {t('dashboard.pendingApprovals')}
                </Heading>
              </CardHeader>
              <CardBody>
                <List spacing={3}>
                  {employeeData.pending_approvals.slice(0, 5).map((approval, index) => (
                    <ListItem key={`approval-${approval.id}-${index}`} display="flex" alignItems="center" p={3} bg="gray.50" borderRadius="md">
                      <ListIcon as={FiBell} color={approval.priority === 'HIGH' ? 'red.500' : 'orange.500'} />
                      <Box flex="1">
                        <Text fontWeight="medium">{approval.request_title || approval.request_code}</Text>
                        <Text fontSize="sm" color="gray.600">
                          {approval.request_type} • {approval.step_name} • {formatCurrency(approval.total_amount)}
                        </Text>
                        <Text fontSize="xs" color="gray.500">
                          {t('common.labels.requestedBy') || 'Dari'}: {approval.requester_name} • {new Date(approval.created_at).toLocaleString('id-ID')}
                        </Text>
                      </Box>
                      <Badge
                        colorScheme={approval.priority === 'HIGH' ? 'red' : 'orange'}
                        size="sm"
                      >
                        {approval.priority || 'PENDING'}
                      </Badge>
                    </ListItem>
                  ))}
                </List>
              </CardBody>
            </Card>
          )}

          {/* Submitted Requests */}
          {employeeData?.submitted_requests && employeeData.submitted_requests.length > 0 && (
            <Card mt={6}>
              <CardHeader>
                <Heading size="md" display="flex" alignItems="center">
                  <Icon as={FiFileText} mr={2} color="blue.500" />
                  {t('purchases.purchaseOrder')}
                </Heading>
              </CardHeader>
              <CardBody>
                <List spacing={3}>
                  {employeeData.submitted_requests.slice(0, 5).map((req, index) => (
                    <ListItem key={`req-${req.id}-${index}`} display="flex" alignItems="center" p={3} bg="gray.50" borderRadius="md">
                      <ListIcon as={FiFileText} color="blue.500" />
                      <Box flex="1">
                        <Text fontWeight="medium">{req.request_title || req.request_code}</Text>
                        <Text fontSize="sm" color="gray.600">
                          {req.current_step_name} • {formatCurrency(req.total_amount)}
                        </Text>
                        <Text fontSize="xs" color="gray.500">
                          {new Date(req.created_at).toLocaleString('id-ID')}
                        </Text>
                      </Box>
                      <Badge
                        colorScheme={req.status === 'APPROVED' ? 'green' : req.status === 'REJECTED' ? 'red' : 'yellow'}
                        size="sm"
                      >
                        {req.status}
                      </Badge>
                    </ListItem>
                  ))}
                </List>
              </CardBody>
            </Card>
          )}

          {/* Recent Notifications */}
          {employeeData?.recent_notifications && employeeData.recent_notifications.length > 0 && (
            <Card mt={6}>
              <CardHeader>
                <Heading size="md" display="flex" alignItems="center">
                  <Icon as={FiActivity} mr={2} color="purple.500" />
                  {t('common.labels.notification') || 'Notifikasi Terbaru'}
                </Heading>
              </CardHeader>
              <CardBody>
                <List spacing={3}>
                  {employeeData.recent_notifications.slice(0, 5).map((notif, index) => (
                    <ListItem key={`notif-${notif.id}-${index}`} display="flex" alignItems="center" p={3} bg={notif.is_read ? 'gray.50' : 'blue.50'} borderRadius="md">
                      <ListIcon as={FiBell} color={notif.is_read ? 'gray.400' : 'blue.500'} />
                      <Box flex="1">
                        <Text fontWeight={notif.is_read ? 'normal' : 'bold'}>{notif.title}</Text>
                        <Text fontSize="sm" color="gray.600">
                          {notif.message}
                        </Text>
                        <Text fontSize="xs" color="gray.500">
                          {new Date(notif.created_at).toLocaleString('id-ID')}
                        </Text>
                      </Box>
                      <Badge
                        colorScheme={notif.priority === 'HIGH' ? 'red' : notif.priority === 'MEDIUM' ? 'orange' : 'gray'}
                        size="sm"
                      >
                        {notif.type}
                      </Badge>
                    </ListItem>
                  ))}
                </List>
              </CardBody>
            </Card>
          )}

          {/* Akses Cepat - Employee Features */}
          <Card mt={6}>
            <CardHeader>
              <Heading size="md" display="flex" alignItems="center">
                <Icon as={FiPlus} mr={2} color="blue.500" />
                {t('dashboard.quickAccess.title')}
              </Heading>
            </CardHeader>
            <CardBody>
              <HStack spacing={4} flexWrap="wrap">
                <Button
                  leftIcon={<FiFileText />}
                  colorScheme="blue"
                  variant="outline"
                  onClick={() => router.push('/purchases')}
                  size="md"
                >
                  {t('purchases.purchaseOrder')}
                </Button>
                <Button
                  leftIcon={<FiActivity />}
                  colorScheme="purple"
                  variant="outline"
                  onClick={() => window.location.reload()}
                  size="md"
                >
                  {t('common.refresh')}
                </Button>
              </HStack>
            </CardBody>
          </Card>
        </>
      )}
    </Box>
  );
};

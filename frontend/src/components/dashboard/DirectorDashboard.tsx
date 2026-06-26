'use client';
import React, { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { 
  Box, 
  Flex, 
  Heading, 
  Text, 
  Button, 
  Card,
  CardHeader,
  CardBody,
  HStack,
  Icon,
  List,
  ListItem,
  Badge,
  useColorModeValue,
} from '@chakra-ui/react';
import {
  FiDollarSign,
  FiShoppingCart,
  FiPlus,
  FiBarChart2,
  FiActivity,
  FiCheckCircle
} from 'react-icons/fi';
import api from '@/services/api';
import { API_ENDPOINTS } from '@/config/api';
import { useTranslation } from '@/hooks/useTranslation';

interface DashboardAnalytics {
  totalSales: number;
  totalPurchases: number;
  accountsReceivable: number;
  accountsPayable: number;
  salesGrowth: number;
  purchasesGrowth: number;
  receivablesGrowth: number;
  payablesGrowth: number;
  recentTransactions: Array<{
    id: number;
    transaction_id: string;
    description: string;
    amount: number;
    date: string;
    type: string;
    status: string;
  }>;
}

interface DirectorDashboardProps {
  analytics: DashboardAnalytics | null;
}

export const DirectorDashboard: React.FC<DirectorDashboardProps> = ({ analytics }) => {
  const router = useRouter();
  const { t } = useTranslation();
  const [approvalStats, setApprovalStats] = useState<{ pending_approvals: number; total_amount_pending: number } | null>(null);
  const [loadingStats, setLoadingStats] = useState<boolean>(false);
  const headingColor = useColorModeValue('gray.800', 'whiteAlpha.900');
  const cardBg = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const mutedTextColor = useColorModeValue('gray.600', 'whiteAlpha.700');

  useEffect(() => {
    const fetchApprovalStats = async () => {
      try {
        setLoadingStats(true);
        const res = await api.get(API_ENDPOINTS.PURCHASES_APPROVAL_STATS);
        setApprovalStats({
          pending_approvals: res.data?.pending_approvals ?? 0,
          total_amount_pending: res.data?.total_amount_pending ?? 0,
        });
      } catch {
        setApprovalStats({ pending_approvals: 0, total_amount_pending: 0 });
      } finally {
        setLoadingStats(false);
      }
    };
    fetchApprovalStats();
  }, []);

  const formatCurrency = (value: number) => new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', minimumFractionDigits: 0 }).format(value || 0);
  const formatPct = (v: number) => `${v >= 0 ? '+' : ''}${(v || 0).toFixed(1)}%`;

  return (
    <Box>
      <Heading as="h2" size="xl" mb={6} color={headingColor}>
        {t('navigation.dashboard')} - {t('users.director')}
      </Heading>

      <Flex gap={4} flexWrap="wrap" mt={2}>
        <Box bg={cardBg} border="1px" borderColor={borderColor} p={4} borderRadius="lg" boxShadow="sm" flex="1" minW="240px">
          <Heading as="h3" size="sm" mb={2}>{t('dashboard.stats.totalRevenue')}</Heading>
          <Text fontSize="xl" fontWeight="bold">{formatCurrency(analytics?.totalSales || 0)}</Text>
          <Text fontSize="sm" color={mutedTextColor}>{formatPct(analytics?.salesGrowth || 0)}</Text>
        </Box>
        <Box bg={cardBg} border="1px" borderColor={borderColor} p={4} borderRadius="lg" boxShadow="sm" flex="1" minW="240px">
          <Heading as="h3" size="sm" mb={2}>{t('dashboard.stats.totalPurchases')}</Heading>
          <Text fontSize="xl" fontWeight="bold">{formatCurrency(analytics?.totalPurchases || 0)}</Text>
          <Text fontSize="sm" color={mutedTextColor}>{formatPct(analytics?.purchasesGrowth || 0)}</Text>
        </Box>
        <Box bg={cardBg} border="1px" borderColor={borderColor} p={4} borderRadius="lg" boxShadow="sm" flex="1" minW="240px">
          <Heading as="h3" size="sm" mb={2}>{t('dashboard.stats.accountsReceivable')}</Heading>
          <Text fontSize="xl" fontWeight="bold">{formatCurrency(analytics?.accountsReceivable || 0)}</Text>
          <Text fontSize="sm" color={mutedTextColor}>{formatPct(analytics?.receivablesGrowth || 0)}</Text>
        </Box>
        <Box bg={cardBg} border="1px" borderColor={borderColor} p={4} borderRadius="lg" boxShadow="sm" flex="1" minW="240px">
          <Heading as="h3" size="sm" mb={2}>{t('dashboard.stats.accountsPayable')}</Heading>
          <Text fontSize="xl" fontWeight="bold">{formatCurrency(analytics?.accountsPayable || 0)}</Text>
          <Text fontSize="sm" color={mutedTextColor}>{formatPct(analytics?.payablesGrowth || 0)}</Text>
        </Box>
        <Box bg={cardBg} border="1px" borderColor={borderColor} p={4} borderRadius="lg" boxShadow="sm" flex="1" minW="240px">
          <Heading as="h3" size="sm" mb={2}>{t('dashboard.pendingApprovals')}</Heading>
          <Text fontSize="xl" fontWeight="bold" display="flex" alignItems="center" gap={2}>
            <Icon as={FiCheckCircle} color="orange.500" />
            {loadingStats ? t('common.loading') : (approvalStats?.pending_approvals ?? 0)} item
          </Text>
          <Text fontSize="sm" color={mutedTextColor}>{t('common.labels.total')}: {formatCurrency(approvalStats?.total_amount_pending || 0)}</Text>
        </Box>
      </Flex>

      {/* Transaksi Terbaru - teks saja */}
      {analytics?.recentTransactions && analytics.recentTransactions.length > 0 && (
        <Card mt={6}>
          <CardHeader>
            <Heading size="md" display="flex" alignItems="center" gap={2}>
              <Icon as={FiActivity} /> {t('dashboard.recentTransactions')}
            </Heading>
          </CardHeader>
          <CardBody>
            <List spacing={3}>
              {analytics.recentTransactions.slice(0, 8).map((txn) => (
                <ListItem key={`${txn.type}-${txn.id}`} display="flex" justifyContent="space-between" gap={4}>
                  <Box>
                    <Text fontWeight="medium">{txn.description || txn.transaction_id}</Text>
                    <Text fontSize="sm" color={mutedTextColor}>{txn.type} • {new Date(txn.date).toLocaleDateString('id-ID')}</Text>
                  </Box>
                  <Badge colorScheme={txn.type === 'SALE' ? 'green' : 'orange'}>{formatCurrency(txn.amount)}</Badge>
                </ListItem>
              ))}
            </List>
          </CardBody>
        </Card>
      )}

      {/* Akses Cepat */}
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
              leftIcon={<FiDollarSign />}
              colorScheme="green"
              variant="outline"
              onClick={() => router.push('/sales')}
              size="md"
            >
              {t('dashboard.quickAccess.addSale')}
            </Button>
            <Button
              leftIcon={<FiShoppingCart />}
              colorScheme="orange"
              variant="outline"
              onClick={() => router.push('/purchases')}
              size="md"
            >
              {t('dashboard.quickAccess.addPurchase')}
            </Button>
            <Button
              leftIcon={<FiBarChart2 />}
              colorScheme="purple"
              variant="outline"
              onClick={() => router.push('/reports')}
              size="md"
            >
              {t('dashboard.quickAccess.financialReports')}
            </Button>
          </HStack>
        </CardBody>
      </Card>
    </Box>
  );
};

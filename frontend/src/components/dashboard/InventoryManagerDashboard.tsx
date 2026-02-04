'use client';
import React, { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import {
  Box,
  Flex,
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
  Badge
} from '@chakra-ui/react';
import {
  FiPackage,
  FiPlus,
  FiBarChart,
} from 'react-icons/fi';
import api from '@/services/api';
import { API_ENDPOINTS } from '@/config/api';
import { useTranslation } from '@/hooks/useTranslation';

export const InventoryManagerDashboard = () => {
  const router = useRouter();
  const { t } = useTranslation();
  const [valuation, setValuation] = useState<number>(0);
  const [lowStock, setLowStock] = useState<{ count: number; items: Array<{ id: number; name: string; stock: number; min_stock: number }> }>({ count: 0, items: [] });
  const [loading, setLoading] = useState<boolean>(true);

  useEffect(() => {
    const load = async () => {
      try {
        setLoading(true);
        const [valRes, lowRes] = await Promise.all([
          api.get(API_ENDPOINTS.INVENTORY_VALUATION),
          api.get(API_ENDPOINTS.INVENTORY_LOW_STOCK),
        ]);
        const totalValue = valRes.data?.data?.total_value ?? 0;
        setValuation(totalValue);
        const items = Array.isArray(lowRes.data?.data) ? lowRes.data.data : [];
        setLowStock({ count: lowRes.data?.count ?? items.length, items: items.slice(0, 5) });
      } catch (_) {
        setValuation(0);
        setLowStock({ count: 0, items: [] });
      } finally {
        setLoading(false);
      }
    };
    load();
  }, []);

  const formatCurrency = (value: number) => new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', minimumFractionDigits: 0 }).format(value || 0);

  return (
    <Box>
      <Heading as="h2" size="xl" mb={6} color="gray.800">
        {t('navigation.dashboard')} - {t('users.inventoryManager')}
      </Heading>

      <Flex gap={4} flexWrap="wrap" mt={2}>
        <Box bg="white" p={4} borderRadius="lg" boxShadow="sm" flex="1" minW="240px">
          <Heading as="h3" size="sm" mb={2}>{t('reports.inventory.totalValue')}</Heading>
          <Text fontSize="xl" fontWeight="bold">{loading ? t('common.loading') : formatCurrency(valuation)}</Text>
        </Box>
        <Box bg="white" p={4} borderRadius="lg" boxShadow="sm" flex="1" minW="240px">
          <Heading as="h3" size="sm" mb={2}>{t('reports.inventory.lowStockAlerts')}</Heading>
          <Text fontSize="xl" fontWeight="bold">{loading ? t('common.loading') : `${lowStock.count} ${t('common.item')}`}</Text>
          {lowStock.items.length > 0 && (
            <Text fontSize="sm" color="gray.600">Top 5: {lowStock.items.map(i => i.name).join(', ')}</Text>
          )}
        </Box>
      </Flex>

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
              leftIcon={<FiPackage />}
              colorScheme="purple"
              variant="outline"
              onClick={() => router.push('/products')}
              size="md"
            >
              {t('dashboard.quickAccess.manageProducts')}
            </Button>
            <Button
              leftIcon={<FiBarChart />}
              colorScheme="teal"
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

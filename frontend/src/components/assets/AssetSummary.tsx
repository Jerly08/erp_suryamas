'use client';

import React from 'react';
import {
  Box,
  SimpleGrid,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  StatArrow,
  Card,
  CardBody,
  Icon,
  Flex,
  Text,
} from '@chakra-ui/react';
import { FiDollarSign, FiTrendingUp, FiTrendingDown, FiPackage } from 'react-icons/fi';
import { AssetsSummary } from '@/types/asset';
import { useTranslation } from '@/hooks/useTranslation';

interface AssetSummaryProps {
  summary: AssetsSummary | null;
  isLoading?: boolean;
}

const formatCurrency = (amount: number) => {
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(amount);
};

const AssetSummaryComponent: React.FC<AssetSummaryProps> = ({ summary, isLoading }) => {
  const { t } = useTranslation();
  
  if (isLoading) {
    return (
      <SimpleGrid columns={{ base: 1, md: 2, lg: 4 }} spacing={6} mb={8}>
        {[1, 2, 3, 4].map((i) => (
          <Card key={i} variant="outline">
            <CardBody>
              <Flex align="center">
                <Box
                  p={3}
                  rounded="md"
                  bg="gray.100"
                  mr={4}
                >
                  <Icon as={FiPackage} boxSize={6} color="gray.400" />
                </Box>
                <Box>
                  <Text fontSize="sm" color="gray.500" mb={2}>
                    {t('common.loading')}
                  </Text>
                  <Text fontSize="2xl" fontWeight="bold" color="gray.300">
                    ---
                  </Text>
                </Box>
              </Flex>
            </CardBody>
          </Card>
        ))}
      </SimpleGrid>
    );
  }

  if (!summary) {
    return null;
  }

  const depreciationPercentage = summary.total_value > 0 
    ? (summary.total_depreciation / summary.total_value) * 100 
    : 0;

  const summaryCards = [
    {
      label: t('assets.summary.totalAssets'),
      value: summary.total_assets.toString(),
      icon: FiPackage,
      color: 'blue',
      helpText: `${summary.active_assets} ${t('assets.summary.activeAssets')}`,
    },
    {
      label: t('assets.summary.totalValue'),
      value: formatCurrency(summary.total_value),
      icon: FiDollarSign,
      color: 'green',
      helpText: t('assets.summary.originalPurchaseValue'),
    },
    {
      label: t('assets.summary.netBookValue'),
      value: formatCurrency(summary.net_book_value),
      icon: FiTrendingUp,
      color: 'purple',
      helpText: t('assets.summary.currentBookValue'),
    },
    {
      label: t('assets.summary.totalDepreciation'),
      value: formatCurrency(summary.total_depreciation),
      icon: FiTrendingDown,
      color: 'red',
      helpText: `${depreciationPercentage.toFixed(1)}% ${t('assets.summary.depreciated')}`,
    },
  ];

  return (
    <SimpleGrid columns={{ base: 1, md: 2, lg: 4 }} spacing={6} mb={8}>
      {summaryCards.map((card, index) => (
        <Card key={index} variant="outline" _hover={{ shadow: 'md' }} transition="all 0.2s">
          <CardBody>
            <Stat>
              <Flex align="center" mb={2}>
                <Box
                  p={3}
                  rounded="md"
                  bg={`${card.color}.50`}
                  mr={4}
                >
                  <Icon as={card.icon} boxSize={6} color={`${card.color}.500`} />
                </Box>
                <Box>
                  <StatLabel fontSize="sm" color="gray.600">
                    {card.label}
                  </StatLabel>
                  <StatNumber fontSize="xl" fontWeight="bold">
                    {card.value}
                  </StatNumber>
                </Box>
              </Flex>
              <StatHelpText fontSize="xs" color="gray.500" mb={0}>
                {card.helpText}
              </StatHelpText>
            </Stat>
          </CardBody>
        </Card>
      ))}
    </SimpleGrid>
  );
};

export default AssetSummaryComponent;

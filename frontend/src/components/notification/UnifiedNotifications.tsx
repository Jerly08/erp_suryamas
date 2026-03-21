'use client';

import React, { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import {
  Alert,
  AlertIcon,
  Badge,
  Box,
  Button,
  Divider,
  HStack,
  IconButton,
  Menu,
  MenuButton,
  MenuItem,
  MenuList,
  Tab,
  TabList,
  TabPanel,
  TabPanels,
  Tabs,
  Text,
  useColorModeValue,
  useToast,
  VStack,
} from '@chakra-ui/react';
import {
  FiAlertCircle,
  FiAlertTriangle,
  FiBell,
  FiCheckCircle,
  FiClock,
  FiPackage,
  FiShoppingCart,
  FiTrash2,
  FiXCircle,
} from 'react-icons/fi';
import axios from 'axios';
import { useTranslation } from '@/hooks/useTranslation';
import { useAuth } from '@/contexts/AuthContext';
import { API_BASE_URL, API_ENDPOINTS } from '@/config/api';
import { formatIDR } from '@/utils/currency';
import AuthExpiredModal from '@/components/auth/AuthExpiredModal';
import approvalService from '../../services/approvalService';

interface NotificationItem {
  id: number;
  type: string;
  title: string;
  message: string;
  priority: string;
  is_read: boolean;
  created_at: string;
  data?: unknown;
}

interface StockAlert {
  id: number;
  product_id: number;
  product_name: string;
  product_code: string;
  current_stock: number;
  threshold_stock: number;
  alert_type: string;
  urgency: string;
  message: string;
  category_name?: string;
}

interface NotificationSummary {
  approval_unread: number;
  stock_unread: number;
  total_unread: number;
}

type NotificationData = Record<string, unknown>;

const APPROVAL_NOTIFICATION_TYPES = [
  'APPROVAL_PENDING',
  'APPROVAL_APPROVED',
  'APPROVAL_REJECTED',
];

const STOCK_NOTIFICATION_TYPES = [
  'LOW_STOCK',
  'REORDER_ALERT',
  'STOCK_OUT',
];

const UnifiedNotifications: React.FC = () => {
  const { t, language } = useTranslation();
  const { user, token, isLoading, logout } = useAuth();
  const router = useRouter();
  const toast = useToast();
  const [approvalNotifications, setApprovalNotifications] = useState<NotificationItem[]>([]);
  const [stockNotifications, setStockNotifications] = useState<NotificationItem[]>([]);
  const [stockAlerts, setStockAlerts] = useState<StockAlert[]>([]);
  const [approvalUnreadCount, setApprovalUnreadCount] = useState(0);
  const [stockUnreadCount, setStockUnreadCount] = useState(0);
  const [isAuthExpiredOpen, setIsAuthExpiredOpen] = useState(false);

  const canViewStockNotifications =
    user?.role === 'admin' ||
    user?.role === 'inventory_manager' ||
    user?.role === 'director';
  const canDismissStockAlerts =
    user?.role === 'admin' ||
    user?.role === 'inventory_manager';
  const normalizedRole = String(user?.role || '').toLowerCase();
  const isEmployeeRole = normalizedRole === 'employee';
  const isFinanceRole = normalizedRole === 'finance';
  const isAuthReady = !isLoading;
  const hasActiveSession = isAuthReady && !!user && !!token;

  const menuBg = useColorModeValue('white', 'var(--bg-secondary)');
  const menuBorderColor = useColorModeValue('gray.200', 'var(--border-color)');
  const textColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const secondaryTextColor = useColorModeValue('gray.600', 'whiteAlpha.700');
  const mutedTextColor = useColorModeValue('gray.500', 'var(--text-secondary)');
  const itemHoverBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const unreadItemBg = useColorModeValue('blue.50', 'rgba(77, 171, 247, 0.12)');
  const unreadStockItemBg = useColorModeValue('orange.50', 'rgba(221, 107, 32, 0.14)');
  const tabListBg = useColorModeValue('gray.50', 'whiteAlpha.50');
  const tabSelectedBg = useColorModeValue('white', 'var(--bg-primary)');
  const tabSelectedColor = useColorModeValue('blue.600', 'blue.200');
  const stockAlertBg = useColorModeValue('gray.50', 'whiteAlpha.50');
  const warningAlertBg = useColorModeValue('orange.50', 'rgba(221, 107, 32, 0.14)');
  const warningAlertBorderColor = useColorModeValue('orange.200', 'orange.400');
  const unreadApprovalDot = useColorModeValue('blue.500', 'blue.300');
  const unreadStockDot = useColorModeValue('orange.500', 'orange.300');

  const getErrorStatus = (error: unknown) => {
    if (axios.isAxiosError(error)) {
      return error.response?.status;
    }
    return undefined;
  };

  const handleUnauthorized = (error: unknown) => {
    if (getErrorStatus(error) === 401) {
      setIsAuthExpiredOpen(true);
      return true;
    }
    return false;
  };

  const fetchAllNotifications = async () => {
    if (!hasActiveSession) {
      setApprovalNotifications([]);
      setStockNotifications([]);
      setStockAlerts([]);
      setApprovalUnreadCount(0);
      setStockUnreadCount(0);
      return;
    }

    await Promise.all([
      fetchNotificationSummary(),
      fetchApprovalNotifications(),
      canViewStockNotifications ? fetchStockNotifications() : Promise.resolve(),
      canViewStockNotifications ? fetchStockAlerts() : Promise.resolve(),
    ]);
  };

  useEffect(() => {
    if (!hasActiveSession) {
      return;
    }

    void fetchAllNotifications();
    const interval = setInterval(() => {
      void fetchAllNotifications();
    }, 60000);
    return () => clearInterval(interval);
    // Notification refresh intentionally follows user/role changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hasActiveSession, user, canViewStockNotifications]);

  const fetchNotificationSummary = async () => {
    if (!token) return;

    try {
      const response = await axios.get(`${API_BASE_URL}${API_ENDPOINTS.NOTIFICATIONS_SUMMARY}`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      const summary = (response.data.summary || {}) as Partial<NotificationSummary>;
      setApprovalUnreadCount(summary.approval_unread || 0);
      setStockUnreadCount(summary.stock_unread || 0);
    } catch (error: unknown) {
      if (getErrorStatus(error) === 404) {
        await fetchNotificationSummaryFallback();
        return;
      }
      if (!handleUnauthorized(error)) {
        console.error('Failed to fetch notification summary:', error);
      }
      setApprovalUnreadCount(0);
      setStockUnreadCount(0);
    }
  };

  const fetchNotificationSummaryFallback = async () => {
    if (!token) return;

    try {
      const requests = [
        axios.get(`${API_BASE_URL}${API_ENDPOINTS.NOTIFICATIONS_UNREAD_COUNT}`, {
          params: { types: APPROVAL_NOTIFICATION_TYPES.join(',') },
          headers: { Authorization: `Bearer ${token}` }
        }),
      ];

      if (canViewStockNotifications) {
        requests.push(
          axios.get(`${API_BASE_URL}${API_ENDPOINTS.NOTIFICATIONS_UNREAD_COUNT}`, {
            params: { types: STOCK_NOTIFICATION_TYPES.join(',') },
            headers: { Authorization: `Bearer ${token}` }
          })
        );
      }

      const responses = await Promise.all(requests);
      setApprovalUnreadCount(responses[0]?.data.count || 0);
      setStockUnreadCount(canViewStockNotifications ? (responses[1]?.data.count || 0) : 0);
    } catch (error: unknown) {
      if (!handleUnauthorized(error)) {
        console.error('Failed to fetch notification summary fallback:', error);
      }
      setApprovalUnreadCount(0);
      setStockUnreadCount(0);
    }
  };

  const fetchApprovalNotifications = async () => {
    if (!token) return;

    try {
      const listResponse = await axios.get(`${API_BASE_URL}${API_ENDPOINTS.NOTIFICATIONS_APPROVALS}`, {
        params: { limit: 10 },
        headers: { Authorization: `Bearer ${token}` }
      });

      setApprovalNotifications(listResponse.data.notifications || []);
    } catch (error: unknown) {
      if (!handleUnauthorized(error) && getErrorStatus(error) !== 403) {
        console.error('Failed to fetch approval notifications:', error);
      }
      setApprovalNotifications([]);
    }
  };

  const fetchStockNotifications = async () => {
    if (!token) return;

    try {
      const [lowStockResponse, reorderResponse, stockOutResponse] = await Promise.all([
        axios.get(`${API_BASE_URL}${API_ENDPOINTS.NOTIFICATIONS_BY_TYPE('LOW_STOCK')}`, {
          params: { limit: 5 },
          headers: { Authorization: `Bearer ${token}` }
        }),
        axios.get(`${API_BASE_URL}${API_ENDPOINTS.NOTIFICATIONS_BY_TYPE('REORDER_ALERT')}`, {
          params: { limit: 5 },
          headers: { Authorization: `Bearer ${token}` }
        }),
        axios.get(`${API_BASE_URL}${API_ENDPOINTS.NOTIFICATIONS_BY_TYPE('STOCK_OUT')}`, {
          params: { limit: 5 },
          headers: { Authorization: `Bearer ${token}` }
        }),
      ]);

      const lowStockNotifs = lowStockResponse.data.notifications || [];
      const reorderNotifs = reorderResponse.data.notifications || [];
      const stockOutNotifs = stockOutResponse.data.notifications || [];
      const allStockNotifs = [...lowStockNotifs, ...reorderNotifs, ...stockOutNotifs]
        .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
        .slice(0, 10);

      setStockNotifications(allStockNotifs);
    } catch (error: unknown) {
      if (!handleUnauthorized(error)) {
        console.error('Failed to fetch stock notifications:', error);
      }
      setStockNotifications([]);
    }
  };

  const fetchStockAlerts = async () => {
    if (!token) return;

    try {
      const response = await axios.get(`${API_BASE_URL}${API_ENDPOINTS.DASHBOARD_STOCK_ALERTS}`, {
        headers: { Authorization: `Bearer ${token}` }
      });

      if (response.data.data?.alerts) {
        setStockAlerts(response.data.data.alerts);
      } else {
        setStockAlerts(response.data.alerts || []);
      }
    } catch (error: unknown) {
      if (handleUnauthorized(error)) {
        setStockAlerts([]);
        return;
      }
      if (getErrorStatus(error) === 500) {
        console.warn('Stock alerts service temporarily unavailable');
      } else if (getErrorStatus(error) === 403) {
        console.info('User not authorized to view stock alerts');
      } else if (axios.isAxiosError(error)) {
        console.error('Failed to fetch stock alerts:', error.response?.data || error.message);
      } else {
        console.error('Failed to fetch stock alerts:', error);
      }
      setStockAlerts([]);
    }
  };

  const handleMarkApprovalAsRead = async (notificationId: number) => {
    try {
      await approvalService.markNotificationAsRead(notificationId);
      setApprovalNotifications((prev) =>
        prev.map((notification) =>
          notification.id === notificationId
            ? { ...notification, is_read: true }
            : notification
        )
      );
      setApprovalUnreadCount((prev) => Math.max(0, prev - 1));
    } catch (error) {
      console.error('Failed to mark approval notification as read:', error);
    }
  };

  const handleMarkStockAsRead = async (notificationId: number) => {
    if (!token) return;

    try {
      await axios.put(
        `${API_BASE_URL}${API_ENDPOINTS.NOTIFICATIONS_MARK_READ(notificationId)}`,
        {},
        { headers: { Authorization: `Bearer ${token}` } }
      );

      setStockNotifications((prev) =>
        prev.map((notification) =>
          notification.id === notificationId
            ? { ...notification, is_read: true }
            : notification
        )
      );
      setStockUnreadCount((prev) => Math.max(0, prev - 1));
    } catch (error) {
      console.error('Failed to mark stock notification as read:', error);
    }
  };

  const deleteApprovalNotification = async (notification: NotificationItem) => {
    try {
      await approvalService.deleteNotification(notification.id);
      setApprovalNotifications((prev) =>
        prev.filter((item) => item.id !== notification.id)
      );
      if (!notification.is_read) {
        setApprovalUnreadCount((prev) => Math.max(0, prev - 1));
      }
    } catch (error) {
      console.error('Failed to delete approval notification:', error);
      toast({
        title: 'Gagal menghapus notifikasi',
        description: 'Notifikasi approval belum berhasil dihapus.',
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
    }
  };

  const deleteStockNotification = async (notification: NotificationItem) => {
    try {
      await approvalService.deleteNotification(notification.id);
      setStockNotifications((prev) =>
        prev.filter((item) => item.id !== notification.id)
      );
      if (!notification.is_read) {
        setStockUnreadCount((prev) => Math.max(0, prev - 1));
      }
    } catch (error) {
      console.error('Failed to delete stock notification:', error);
      toast({
        title: 'Gagal menghapus notifikasi',
        description: 'Notifikasi stok belum berhasil dihapus.',
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
    }
  };

  const dismissStockAlert = async (alertId: number) => {
    if (!token) return;

    try {
      await axios.post(
        `${API_BASE_URL}${API_ENDPOINTS.DASHBOARD_STOCK_ALERTS_DISMISS(alertId)}`,
        {},
        { headers: { Authorization: `Bearer ${token}` } }
      );

      setStockAlerts((prev) => prev.filter((alert) => alert.id !== alertId));
    } catch (error) {
      console.error('Failed to dismiss stock alert:', error);
    }
  };

  const getApprovalIcon = (type: string) => {
    switch (type) {
      case 'APPROVAL_PENDING':
        return <FiClock color="#dd6b20" />;
      case 'APPROVAL_APPROVED':
        return <FiCheckCircle color="#38a169" />;
      case 'APPROVAL_REJECTED':
        return <FiXCircle color="#e53e3e" />;
      default:
        return <FiShoppingCart color="#3182ce" />;
    }
  };

  const getStockIcon = (urgency: string) => {
    switch (urgency) {
      case 'critical':
        return <FiAlertTriangle color="#e53e3e" />;
      case 'high':
        return <FiAlertCircle color="#dd6b20" />;
      default:
        return <FiPackage color="#3182ce" />;
    }
  };

  const handleAuthExpiredLoginRedirect = () => {
    setIsAuthExpiredOpen(false);
    logout();
  };

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    const now = new Date();
    const diffInHours = (now.getTime() - date.getTime()) / (1000 * 60 * 60);

    if (diffInHours < 1) return t('notifications.time.justNow');
    if (diffInHours < 24) return t('notifications.time.hoursAgo', { count: Math.floor(diffInHours) });

    const locale = language === 'id' ? 'id-ID' : 'en-US';
    return date.toLocaleDateString(locale, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  };

  const parseNotificationData = (data?: NotificationItem['data']): NotificationData | null => {
    if (!data) return null;
    if (typeof data !== 'string' && typeof data === 'object') {
      return data as NotificationData;
    }
    if (typeof data !== 'string') return null;

    try {
      return JSON.parse(data);
    } catch {
      return null;
    }
  };

  const formatMessageCurrency = (message: string, data?: NotificationItem['data']) => {
    if (!message) return '';

    let result = message;
    const parsedData = parseNotificationData(data);

    const totalAmount = parsedData?.total_amount ?? parsedData?.amount ?? parsedData?.totalAmount;
    if (typeof totalAmount === 'number') {
      const amountLabel = t('common.labels.amount');
      result = result.replace(/\(\s*Amount\s*:\s*[^\)]*\)/i, () => `(${amountLabel}: ${formatIDR(totalAmount)})`);
    }

    result = result.replace(/(Amount|amount)\s*:\s*(\d+[\d.,]*)/g, (_match, _label, value) => {
      const normalized = parseFloat(String(value).replace(/\./g, '').replace(/,/g, '.'));
      const formatted = Number.isNaN(normalized) ? value : formatIDR(normalized);
      const amountLabel = t('common.labels.amount');
      return `${amountLabel}: ${formatted}`;
    });

    return result;
  };

  const getNumericValue = (value: unknown) => {
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value;
    }

    if (typeof value === 'string') {
      const parsed = Number(value);
      if (Number.isFinite(parsed)) {
        return parsed;
      }
    }

    return null;
  };

  const getNotificationHref = (notification: NotificationItem) => {
    const data = parseNotificationData(notification.data);
    const entityType = String(data?.entity_type || '').toUpperCase();
    const entityId = getNumericValue(data?.entity_id);
    const purchaseId = getNumericValue(data?.purchase_id) ?? (entityType === 'PURCHASE' ? entityId : null);
    const saleId = getNumericValue(data?.sale_id) ?? (entityType === 'SALE' ? entityId : null);
    const purchaseRequestId =
      getNumericValue(data?.purchase_request_id) ??
      getNumericValue(data?.pr_id) ??
      (entityType === 'PURCHASE_REQUEST' ? entityId : null);
    const productId = getNumericValue(data?.product_id);

    if (purchaseId !== null) {
      return `/purchases?notificationPurchaseId=${purchaseId}`;
    }

    if (saleId !== null) {
      return `/sales/${saleId}`;
    }

    if (purchaseRequestId !== null) {
      return `/purchase-requests?notificationRequestId=${purchaseRequestId}`;
    }

    if (productId !== null) {
      return `/products?notificationProductId=${productId}`;
    }

    if (notification.type.startsWith('APPROVAL_')) {
      return '/purchases';
    }

    if (STOCK_NOTIFICATION_TYPES.includes(notification.type)) {
      return '/products';
    }

    return null;
  };

  const handleApprovalNotificationClick = (notification: NotificationItem) => {
    if (!notification.is_read) {
      void handleMarkApprovalAsRead(notification.id);
    }

    const href = getNotificationHref(notification);
    if (href) {
      router.push(href);
    }
  };

  const handleStockNotificationClick = (notification: NotificationItem) => {
    if (!notification.is_read) {
      void handleMarkStockAsRead(notification.id);
    }

    const href = getNotificationHref(notification);
    if (href) {
      router.push(href);
    }
  };

  const handleStockAlertClick = (alert: StockAlert) => {
    if (alert.product_id) {
      router.push(`/products?notificationProductId=${alert.product_id}`);
      return;
    }

    router.push('/products');
  };

  const getUrgencyColor = (urgency: string) => {
    switch (urgency) {
      case 'critical':
        return 'red';
      case 'high':
        return 'orange';
      case 'medium':
        return 'yellow';
      default:
        return 'blue';
    }
  };

  const getNotificationStatusConfig = (type: string) => {
    switch (type) {
      case 'APPROVAL_PENDING':
        return { label: 'Menunggu', colorScheme: 'orange' };
      case 'APPROVAL_APPROVED':
        return { label: 'Disetujui', colorScheme: 'green' };
      case 'APPROVAL_REJECTED':
        return { label: 'Ditolak', colorScheme: 'red' };
      default:
        return { label: type, colorScheme: 'gray' };
    }
  };

  const getNotificationEntityLabel = (notification: NotificationItem) => {
    const data = parseNotificationData(notification.data);

    const purchaseCode = typeof data?.purchase_code === 'string' ? data.purchase_code : '';
    const prNumber = typeof data?.pr_number === 'string' ? data.pr_number : '';
    const productName = typeof data?.product_name === 'string' ? data.product_name : '';
    const entityType = typeof data?.entity_type === 'string' ? data.entity_type : '';

    if (purchaseCode) return purchaseCode;
    if (prNumber) return prNumber;
    if (productName) return productName;
    if (entityType) return entityType.replace(/_/g, ' ');

    return '';
  };

  const getNotificationAmount = (notification: NotificationItem) => {
    const data = parseNotificationData(notification.data);
    return getNumericValue(data?.amount ?? data?.total_amount ?? data?.totalAmount);
  };

  const getPendingAgeLabel = (notification: NotificationItem) => {
    if (notification.type !== 'APPROVAL_PENDING') {
      return null;
    }

    const createdAt = new Date(notification.created_at);
    const ageHours = Math.floor((Date.now() - createdAt.getTime()) / (1000 * 60 * 60));

    if (ageHours < 24) {
      return null;
    }

    const ageDays = Math.floor(ageHours / 24);
    return ageDays > 0 ? `${ageDays}h` : `${ageHours}j`;
  };

  const getNotificationHint = (notification: NotificationItem) => {
    if (isFinanceRole && notification.type === 'APPROVAL_PENDING') {
      const pendingAge = getPendingAgeLabel(notification);
      return pendingAge ? `Perlu review segera, tertunda ${pendingAge}` : 'Buka detail untuk review dan approval';
    }

    if (isEmployeeRole) {
      switch (notification.type) {
        case 'APPROVAL_APPROVED':
          return 'Pengajuan Anda sudah selesai diproses';
        case 'APPROVAL_REJECTED':
          return 'Pengajuan perlu revisi atau tindak lanjut';
        case 'APPROVAL_PENDING':
          return 'Pengajuan masih menunggu proses persetujuan';
        default:
          return '';
      }
    }

    return '';
  };

  const renderApprovalNotificationItem = (notification: NotificationItem) => {
    const status = getNotificationStatusConfig(notification.type);
    const entityLabel = getNotificationEntityLabel(notification);
    const amount = getNotificationAmount(notification);
    const pendingAge = getPendingAgeLabel(notification);
    const hint = getNotificationHint(notification);

    return (
      <MenuItem
        key={notification.id}
        bg={notification.is_read ? menuBg : unreadItemBg}
        _hover={{ bg: itemHoverBg }}
        onClick={() => handleApprovalNotificationClick(notification)}
      >
        <HStack align="start" spacing={3} w="full">
          <Box pt={1}>{getApprovalIcon(notification.type)}</Box>
          <VStack align="start" spacing={1} flex={1}>
            <HStack justify="space-between" w="full" align="start">
              <Text fontSize="sm" fontWeight={notification.is_read ? 'normal' : 'semibold'} color={textColor}>
                {notification.title}
              </Text>
              <HStack spacing={1} align="center">
                {!notification.is_read && <Box w={2} h={2} bg={unreadApprovalDot} borderRadius="full" />}
                <Box
                  as="span"
                  aria-label="Hapus notifikasi"
                  title="Hapus notifikasi"
                  display="inline-flex"
                  alignItems="center"
                  justifyContent="center"
                  w="22px"
                  h="22px"
                  borderRadius="md"
                  color={mutedTextColor}
                  cursor="pointer"
                  _hover={{ bg: itemHoverBg, color: 'red.400' }}
                  onClick={(event: React.MouseEvent<HTMLSpanElement>) => {
                    event.preventDefault();
                    event.stopPropagation();
                    void deleteApprovalNotification(notification);
                  }}
                >
                  <FiTrash2 />
                </Box>
              </HStack>
            </HStack>

            <HStack spacing={2} flexWrap="wrap">
              <Badge colorScheme={status.colorScheme} fontSize="0.6em">
                {status.label}
              </Badge>
              {entityLabel && (
                <Badge colorScheme="purple" variant="subtle" fontSize="0.6em">
                  {entityLabel}
                </Badge>
              )}
              {amount !== null && (
                <Badge colorScheme="blue" variant="subtle" fontSize="0.6em">
                  {formatIDR(amount)}
                </Badge>
              )}
              {pendingAge && (
                <Badge colorScheme="red" variant="outline" fontSize="0.6em">
                  {pendingAge}
                </Badge>
              )}
            </HStack>

            <Text fontSize="sm" color={secondaryTextColor} noOfLines={2}>
              {formatMessageCurrency(notification.message, notification.data)}
            </Text>

            {hint && (
              <Text fontSize="xs" color={mutedTextColor}>
                {hint}
              </Text>
            )}

            <HStack spacing={2}>
              <Text fontSize="xs" color={mutedTextColor}>
                {formatDate(notification.created_at)}
              </Text>
              <Badge colorScheme="gray" fontSize="0.6em">
                {notification.priority}
              </Badge>
            </HStack>
          </VStack>
        </HStack>
      </MenuItem>
    );
  };

  const totalUnreadCount = approvalUnreadCount + stockUnreadCount;
  const financeActionNotifications = approvalNotifications.filter((notification) => notification.type === 'APPROVAL_PENDING');
  const financeInfoNotifications = approvalNotifications.filter((notification) => notification.type !== 'APPROVAL_PENDING');
  const employeeActiveNotifications = approvalNotifications.filter((notification) => !notification.is_read);
  const employeeHistoryNotifications = approvalNotifications.filter((notification) => notification.is_read);
  const financeActionUnreadCount = financeActionNotifications.filter((notification) => !notification.is_read).length;
  const financeInfoUnreadCount = financeInfoNotifications.filter((notification) => !notification.is_read).length;

  return (
    <>
      <AuthExpiredModal
        isOpen={isAuthExpiredOpen}
        onLoginRedirect={handleAuthExpiredLoginRedirect}
      />

      <Menu placement="bottom-end" autoSelect={false} onOpen={fetchAllNotifications}>
        <MenuButton as={IconButton} aria-label="Notifications" variant="ghost" color={textColor}>
          <Box position="relative">
            <FiBell />
            {totalUnreadCount > 0 && (
              <Badge
                colorScheme="red"
                borderRadius="full"
                position="absolute"
                top={-2}
                right={-2}
                fontSize="0.6em"
                px={2}
              >
                {totalUnreadCount > 99 ? '99+' : totalUnreadCount}
              </Badge>
            )}
          </Box>
        </MenuButton>

        <MenuList
          minW="420px"
          maxW="90vw"
          maxH="80vh"
          overflowY="auto"
          bg={menuBg}
          borderColor={menuBorderColor}
          color={textColor}
          boxShadow="xl"
        >
          <Box px={3} pt={2} pb={1}>
            <Text fontSize="md" fontWeight="bold" color={textColor}>
              {t('notifications.title')}
            </Text>
            <Text fontSize="xs" color={mutedTextColor}>
              {isEmployeeRole
                ? 'Pantau perubahan status pengajuan Anda'
                : isFinanceRole
                  ? 'Fokus pada item yang perlu tindakan lebih dulu'
                  : totalUnreadCount > 0
                    ? t(totalUnreadCount > 1 ? 'notifications.unread_plural' : 'notifications.unread', { count: totalUnreadCount })
                    : t('notifications.title')}
            </Text>
          </Box>

          <Divider borderColor={menuBorderColor} />

          {canViewStockNotifications ? (
            <Tabs variant="enclosed" size="sm">
              <TabList bg={tabListBg} borderBottomColor={menuBorderColor}>
                <Tab color={mutedTextColor} _selected={{ bg: tabSelectedBg, color: tabSelectedColor, borderColor: menuBorderColor }}>
                  {t('notifications.tabs.approval')}
                  {approvalUnreadCount > 0 && (
                    <Badge ml={2} colorScheme="blue" borderRadius="full" fontSize="0.6em">
                      {approvalUnreadCount}
                    </Badge>
                  )}
                </Tab>
                <Tab color={mutedTextColor} _selected={{ bg: tabSelectedBg, color: tabSelectedColor, borderColor: menuBorderColor }}>
                  {t('notifications.tabs.stock')}
                  {stockUnreadCount > 0 && (
                    <Badge ml={2} colorScheme="red" borderRadius="full" fontSize="0.6em">
                      {stockUnreadCount}
                    </Badge>
                  )}
                </Tab>
              </TabList>

              <TabPanels>
                <TabPanel p={0}>
                  <Box maxH="300px" overflowY="auto">
                    {approvalNotifications.length === 0 ? (
                      <MenuItem isDisabled bg={menuBg}>
                        <Text fontSize="sm" color={mutedTextColor}>
                          {t('notifications.empty.approval')}
                        </Text>
                      </MenuItem>
                    ) : (
                      approvalNotifications.map((notification) => renderApprovalNotificationItem(notification))
                    )}
                  </Box>
                </TabPanel>

                <TabPanel p={0}>
                  <Box maxH="300px" overflowY="auto">
                    {stockAlerts.length === 0 && stockNotifications.length === 0 ? (
                      <MenuItem isDisabled bg={menuBg}>
                        <Text fontSize="sm" color={mutedTextColor}>
                          {t('notifications.empty.stock')}
                        </Text>
                      </MenuItem>
                    ) : (
                      <>
                        {stockAlerts.map((alert) => (
                          <Box
                            key={`alert-${alert.id}`}
                            p={3}
                            borderBottom="1px"
                            borderColor={menuBorderColor}
                            bg={stockAlertBg}
                            cursor="pointer"
                            _hover={{ bg: itemHoverBg }}
                            onClick={() => handleStockAlertClick(alert)}
                          >
                            <HStack align="start" spacing={3}>
                              <Box pt={1}>{getStockIcon(alert.urgency)}</Box>
                              <VStack align="start" spacing={1} flex={1}>
                                <HStack justify="space-between" w="full">
                                  <Text fontSize="sm" fontWeight="bold" color={textColor}>
                                    {alert.product_name}
                                  </Text>
                                  <Badge colorScheme={getUrgencyColor(alert.urgency)} fontSize="0.6em">
                                    {alert.urgency}
                                  </Badge>
                                </HStack>

                                <Text fontSize="xs" color={secondaryTextColor}>
                                  {t('notifications.stock.code')}: {alert.product_code}
                                  {alert.category_name ? ` - ${alert.category_name}` : ''}
                                </Text>

                                <Alert
                                  status="warning"
                                  size="sm"
                                  borderRadius="md"
                                  bg={warningAlertBg}
                                  border="1px solid"
                                  borderColor={warningAlertBorderColor}
                                >
                                  <AlertIcon />
                                  <Text fontSize="xs" color={textColor}>
                                    {t('notifications.stock.current')}: {alert.current_stock} / {t('notifications.stock.min')}: {alert.threshold_stock}
                                  </Text>
                                </Alert>

                                {canDismissStockAlerts && (
                                  <Button
                                    size="xs"
                                    variant="outline"
                                    borderColor={menuBorderColor}
                                    color={textColor}
                                    _hover={{ bg: itemHoverBg }}
                                    onClick={(event) => {
                                      event.stopPropagation();
                                      void dismissStockAlert(alert.id);
                                    }}
                                  >
                                    {t('notifications.stock.dismiss')}
                                  </Button>
                                )}
                              </VStack>
                            </HStack>
                          </Box>
                        ))}

                        {stockNotifications.map((notification) => {
                          const data = parseNotificationData(notification.data) as Record<string, unknown> | null;
                          return (
                            <MenuItem
                              key={`notif-${notification.id}`}
                              bg={notification.is_read ? menuBg : unreadStockItemBg}
                              _hover={{ bg: itemHoverBg }}
                              onClick={() => handleStockNotificationClick(notification)}
                            >
                              <HStack align="start" spacing={3} w="full">
                                <Box pt={1}>
                                  <FiAlertTriangle color="#dd6b20" />
                                </Box>
                                <VStack align="start" spacing={0} flex={1}>
                                  <HStack justify="space-between" w="full">
                                    <Text fontSize="sm" fontWeight={notification.is_read ? 'normal' : 'semibold'} color={textColor}>
                                      {notification.title}
                                    </Text>
                                    <HStack spacing={1} align="center">
                                      {!notification.is_read && <Box w={2} h={2} bg={unreadStockDot} borderRadius="full" />}
                                      <Box
                                        as="span"
                                        aria-label="Hapus notifikasi"
                                        title="Hapus notifikasi"
                                        display="inline-flex"
                                        alignItems="center"
                                        justifyContent="center"
                                        w="22px"
                                        h="22px"
                                        borderRadius="md"
                                        color={mutedTextColor}
                                        cursor="pointer"
                                        _hover={{ bg: itemHoverBg, color: 'red.400' }}
                                        onClick={(event: React.MouseEvent<HTMLSpanElement>) => {
                                          event.preventDefault();
                                          event.stopPropagation();
                                          void deleteStockNotification(notification);
                                        }}
                                      >
                                        <FiTrash2 />
                                      </Box>
                                    </HStack>
                                  </HStack>

                                  <Text fontSize="xs" color={secondaryTextColor} noOfLines={2}>
                                    {formatMessageCurrency(notification.message, notification.data)}
                                  </Text>

                                  {data && (
                                    <Text fontSize="xs" color={mutedTextColor}>
                                      {t('notifications.stock.current')}: {String(data.current_stock ?? '-')} / {t('notifications.stock.min')}: {String(data.minimum_stock ?? data.reorder_level ?? '-')}
                                    </Text>
                                  )}

                                  <Text fontSize="xs" color={mutedTextColor}>
                                    {formatDate(notification.created_at)}
                                  </Text>
                                </VStack>
                              </HStack>
                            </MenuItem>
                          );
                        })}
                      </>
                    )}
                  </Box>
                </TabPanel>
              </TabPanels>
            </Tabs>
          ) : isEmployeeRole ? (
            <Tabs variant="enclosed" size="sm">
              <TabList bg={tabListBg} borderBottomColor={menuBorderColor}>
                <Tab color={mutedTextColor} _selected={{ bg: tabSelectedBg, color: tabSelectedColor, borderColor: menuBorderColor }}>
                  Status Pengajuan
                  {approvalUnreadCount > 0 && (
                    <Badge ml={2} colorScheme="blue" borderRadius="full" fontSize="0.6em">
                      {approvalUnreadCount}
                    </Badge>
                  )}
                </Tab>
                <Tab color={mutedTextColor} _selected={{ bg: tabSelectedBg, color: tabSelectedColor, borderColor: menuBorderColor }}>
                  Riwayat
                </Tab>
              </TabList>

              <TabPanels>
                <TabPanel p={0}>
                  <Box maxH="320px" overflowY="auto">
                    {employeeActiveNotifications.length === 0 ? (
                      <MenuItem isDisabled bg={menuBg}>
                        <Text fontSize="sm" color={mutedTextColor}>
                          Belum ada perubahan status terbaru
                        </Text>
                      </MenuItem>
                    ) : (
                      employeeActiveNotifications.map((notification) => renderApprovalNotificationItem(notification))
                    )}
                  </Box>
                </TabPanel>

                <TabPanel p={0}>
                  <Box maxH="320px" overflowY="auto">
                    {employeeHistoryNotifications.length === 0 ? (
                      <MenuItem isDisabled bg={menuBg}>
                        <Text fontSize="sm" color={mutedTextColor}>
                          Riwayat notifikasi belum ada
                        </Text>
                      </MenuItem>
                    ) : (
                      employeeHistoryNotifications.map((notification) => renderApprovalNotificationItem(notification))
                    )}
                  </Box>
                </TabPanel>
              </TabPanels>
            </Tabs>
          ) : isFinanceRole ? (
            <Tabs variant="enclosed" size="sm">
              <TabList bg={tabListBg} borderBottomColor={menuBorderColor}>
                <Tab color={mutedTextColor} _selected={{ bg: tabSelectedBg, color: tabSelectedColor, borderColor: menuBorderColor }}>
                  Perlu Tindakan
                  {financeActionUnreadCount > 0 && (
                    <Badge ml={2} colorScheme="orange" borderRadius="full" fontSize="0.6em">
                      {financeActionUnreadCount}
                    </Badge>
                  )}
                </Tab>
                <Tab color={mutedTextColor} _selected={{ bg: tabSelectedBg, color: tabSelectedColor, borderColor: menuBorderColor }}>
                  Informasi
                  {financeInfoUnreadCount > 0 && (
                    <Badge ml={2} colorScheme="blue" borderRadius="full" fontSize="0.6em">
                      {financeInfoUnreadCount}
                    </Badge>
                  )}
                </Tab>
              </TabList>

              <TabPanels>
                <TabPanel p={0}>
                  <Box px={3} py={2} bg={warningAlertBg} borderBottom="1px" borderColor={menuBorderColor}>
                    <Text fontSize="xs" color={secondaryTextColor}>
                      Prioritaskan approval yang paling lama tertunda terlebih dahulu.
                    </Text>
                  </Box>
                  <Box maxH="300px" overflowY="auto">
                    {financeActionNotifications.length === 0 ? (
                      <MenuItem isDisabled bg={menuBg}>
                        <Text fontSize="sm" color={mutedTextColor}>
                          Tidak ada approval yang perlu tindakan
                        </Text>
                      </MenuItem>
                    ) : (
                      financeActionNotifications.map((notification) => renderApprovalNotificationItem(notification))
                    )}
                  </Box>
                </TabPanel>

                <TabPanel p={0}>
                  <Box maxH="320px" overflowY="auto">
                    {financeInfoNotifications.length === 0 ? (
                      <MenuItem isDisabled bg={menuBg}>
                        <Text fontSize="sm" color={mutedTextColor}>
                          Belum ada notifikasi informasional
                        </Text>
                      </MenuItem>
                    ) : (
                      financeInfoNotifications.map((notification) => renderApprovalNotificationItem(notification))
                    )}
                  </Box>
                </TabPanel>
              </TabPanels>
            </Tabs>
          ) : (
            <Box maxH="350px" overflowY="auto">
              {approvalNotifications.length === 0 ? (
                <MenuItem isDisabled bg={menuBg}>
                    <Text fontSize="sm" color={mutedTextColor}>
                      {t('notifications.empty.general')}
                    </Text>
                  </MenuItem>
                ) : (
                  approvalNotifications.map((notification) => renderApprovalNotificationItem(notification))
              )}
            </Box>
          )}
        </MenuList>
      </Menu>
    </>
  );
};

export default UnifiedNotifications;

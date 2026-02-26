'use client';

import React, { useEffect, useState } from 'react';
import {
  Box,
  IconButton,
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  Divider,
  Text,
  HStack,
  VStack,
  Badge,
  Spinner,
  Alert,
  AlertIcon,
  Button,
  Tabs,
  TabList,
  TabPanels,
  Tab,
  TabPanel,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  useDisclosure,
  AlertDialog,
  AlertDialogBody,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogContent,
  AlertDialogOverlay,
} from '@chakra-ui/react';
import {
  FiBell,
  FiCheckCircle,
  FiXCircle,
  FiClock,
  FiShoppingCart,
  FiPackage,
  FiAlertTriangle,
  FiAlertCircle,
  FiLogOut
} from 'react-icons/fi';
import { useTranslation } from '@/hooks/useTranslation';
import { useAuth } from '@/contexts/AuthContext';
import { useRouter } from 'next/navigation';
import approvalService from '../../services/approvalService';
import axios from 'axios';
import { API_BASE_URL, API_ENDPOINTS } from '@/config/api';
import { formatIDR } from '@/utils/currency';

interface NotificationItem {
  id: number;
  type: string;
  title: string;
  message: string;
  priority: string;
  is_read: boolean;
  created_at: string;
  data?: any;
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

const UnifiedNotifications: React.FC = () => {
  const { t, language } = useTranslation();
  const [approvalNotifications, setApprovalNotifications] = useState<NotificationItem[]>([]);
  const [stockNotifications, setStockNotifications] = useState<NotificationItem[]>([]);
  const [stockAlerts, setStockAlerts] = useState<StockAlert[]>([]);
  const [approvalUnreadCount, setApprovalUnreadCount] = useState(0);
  const [stockUnreadCount, setStockUnreadCount] = useState(0);
  const [loading, setLoading] = useState(false);
  const [authError, setAuthError] = useState(false);
  const { user, logout } = useAuth();
  const router = useRouter();
  const { isOpen: isAuthErrorOpen, onOpen: onAuthErrorOpen, onClose: onAuthErrorClose } = useDisclosure();
  const cancelRef = React.useRef<HTMLButtonElement>(null);

  // Determine if user can see stock notifications
  const canViewStockNotifications = user?.role === 'admin' || user?.role === 'inventory_manager';

  useEffect(() => {
    fetchAllNotifications();
    const interval = setInterval(fetchAllNotifications, 60000); // Refresh every minute
    return () => clearInterval(interval);
  }, [user]);

  const fetchAllNotifications = async () => {
    await Promise.all([
      fetchApprovalNotifications(),
      canViewStockNotifications && fetchStockNotifications(),
      canViewStockNotifications && fetchStockAlerts(),
    ].filter(Boolean));
  };

  const fetchApprovalNotifications = async () => {
    try {
      const token = localStorage.getItem('token');
      const response = await axios.get(`${API_BASE_URL}${API_ENDPOINTS.NOTIFICATIONS_APPROVALS}`, {
        params: { limit: 10 },
        headers: { Authorization: `Bearer ${token}` }
      });
      setApprovalNotifications(response.data.notifications || []);

      // Count unread approval notifications from the response
      const unreadCount = (response.data.notifications || []).filter((n: NotificationItem) => !n.is_read).length;
      setApprovalUnreadCount(unreadCount);
    } catch (error: any) {
      if (error.response?.status === 401) {
        // Token expired or invalid - show user-friendly modal
        setAuthError(true);
        onAuthErrorOpen();
      } else if (error.response?.status !== 403) {
        console.error('Failed to fetch approval notifications:', error);
      }
      setApprovalNotifications([]);
      setApprovalUnreadCount(0);
    }
  };

  const fetchStockNotifications = async () => {
    try {
      const token = localStorage.getItem('token');

      // Get MIN_STOCK, REORDER_ALERT, and STOCK_OUT notifications
      const [minStockResponse, reorderResponse, stockOutResponse] = await Promise.all([
        axios.get(`${API_BASE_URL}${API_ENDPOINTS.NOTIFICATIONS_BY_TYPE('MIN_STOCK')}`, {
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
        })
      ]);

      // Combine all types of notifications
      const minStockNotifs = minStockResponse.data.notifications || [];
      const reorderNotifs = reorderResponse.data.notifications || [];
      const stockOutNotifs = stockOutResponse.data.notifications || [];
      const allStockNotifs = [...minStockNotifs, ...reorderNotifs, ...stockOutNotifs]
        .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
        .slice(0, 10); // Limit to 10 most recent

      setStockNotifications(allStockNotifs);

      // Count unread notifications from both types
      const unreadCount = allStockNotifs.filter((n: NotificationItem) => !n.is_read).length;
      setStockUnreadCount(unreadCount);

    } catch (error: any) {
      if (error.response?.status === 401) {
        // Token expired or invalid - show user-friendly modal
        setAuthError(true);
        onAuthErrorOpen();
      } else {
        console.error('Failed to fetch stock notifications:', error);
      }
      setStockNotifications([]);
      setStockUnreadCount(0);
    }
  };

  const fetchStockAlerts = async () => {
    try {
      const token = localStorage.getItem('token');
      const response = await axios.get(`${API_BASE_URL}${API_ENDPOINTS.DASHBOARD_STOCK_ALERTS}`, {
        headers: { Authorization: `Bearer ${token}` }
      });

      if (response.data.data?.alerts) {
        setStockAlerts(response.data.data.alerts);
      } else {
        // Handle different response structure
        setStockAlerts(response.data.alerts || []);
      }
    } catch (error: any) {
      // Handle specific error cases
      if (error.response?.status === 401) {
        // Token expired or invalid - show user-friendly modal
        setAuthError(true);
        onAuthErrorOpen();
      } else if (error.response?.status === 500) {
        console.warn('Stock alerts service temporarily unavailable');
      } else if (error.response?.status === 403) {
        console.info('User not authorized to view stock alerts');
      } else {
        console.error('Failed to fetch stock alerts:', error.response?.data || error.message);
      }
      setStockAlerts([]);
    }
  };

  const handleMarkApprovalAsRead = async (notificationId: number) => {
    try {
      await approvalService.markNotificationAsRead(notificationId);
      setApprovalNotifications(prev => prev.map(n =>
        n.id === notificationId ? { ...n, is_read: true } : n
      ));
      setApprovalUnreadCount(prev => Math.max(0, prev - 1));
    } catch (error) {
      console.error('Failed to mark approval notification as read:', error);
    }
  };

  const handleMarkStockAsRead = async (notificationId: number) => {
    try {
      const token = localStorage.getItem('token');
      await axios.put(
        `${API_BASE_URL}${API_ENDPOINTS.NOTIFICATIONS_MARK_READ(notificationId)}`,
        {},
        { headers: { Authorization: `Bearer ${token}` } }
      );

      setStockNotifications(prev =>
        prev.map(n => n.id === notificationId ? { ...n, is_read: true } : n)
      );
      setStockUnreadCount(prev => Math.max(0, prev - 1));
    } catch (error) {
      console.error('Failed to mark stock notification as read:', error);
    }
  };

  const dismissStockAlert = async (alertId: number) => {
    try {
      const token = localStorage.getItem('token');
      await axios.post(
        `${API_BASE_URL}${API_ENDPOINTS.DASHBOARD_STOCK_ALERTS_DISMISS(alertId)}`,
        {},
        { headers: { Authorization: `Bearer ${token}` } }
      );

      setStockAlerts(prev => prev.filter(a => a.id !== alertId));
    } catch (error) {
      console.error('Failed to dismiss stock alert:', error);
    }
  };

  const getApprovalIcon = (type: string) => {
    switch (type) {
      case 'approval_pending':
        return <FiClock color="#dd6b20" />;
      case 'approval_approved':
        return <FiCheckCircle color="#38a169" />;
      case 'approval_rejected':
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

  const handleLogout = () => {
    onAuthErrorClose();
    logout();
    router.push('/login');
  };

  const handleCloseAuthError = () => {
    setAuthError(false);
    onAuthErrorClose();
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

  // Replace any inline numeric amount in message with formatted IDR
  const formatMessageCurrency = (message: string, data?: any) => {
    if (!message) return '';

    let result = message;

    // If structured data contains total_amount, ensure we append a nicely formatted value
    const totalAmount = data?.total_amount ?? data?.amount ?? data?.totalAmount;
    if (typeof totalAmount === 'number') {
      const amountLabel = t('common.labels.amount');
      // Replace patterns like (Amount: ...)
      result = result.replace(/\(\s*Amount\s*:\s*[^\)]*\)/i, (m) => `(${amountLabel}: ${formatIDR(totalAmount)})`);
    }

    // Generic replacement: find "Amount: <number>" or "amount: <number>"
    result = result.replace(/(Amount|amount)\s*:\s*(\d+[\d.,]*)/g, (_m, lbl, num) => {
      // Normalize number
      const normalized = parseFloat(String(num).replace(/\./g, '').replace(/,/g, '.'));
      const formatted = isNaN(normalized) ? num : formatIDR(normalized);
      const amountLabel = t('common.labels.amount');
      return `${amountLabel}: ${formatted}`;
    });

    return result;
  };

  const getUrgencyColor = (urgency: string) => {
    switch (urgency) {
      case 'critical': return 'red';
      case 'high': return 'orange';
      case 'medium': return 'yellow';
      default: return 'blue';
    }
  };

  const totalUnreadCount = approvalUnreadCount + stockUnreadCount + stockAlerts.length;

  return (
    <Menu placement="bottom-end" autoSelect={false} onOpen={fetchAllNotifications}>
      <MenuButton as={IconButton} aria-label="Notifications" variant="ghost">
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

      <MenuList minW="420px" maxW="90vw" maxH="80vh" overflowY="auto">
        <Box px={3} pt={2} pb={1}>
          <Text fontSize="md" fontWeight="bold">{t('notifications.title')}</Text>
          {totalUnreadCount > 0 && (
            <Text fontSize="xs" color="gray.500">
              {t(totalUnreadCount > 1 ? 'notifications.unread_plural' : 'notifications.unread', { count: totalUnreadCount })}
            </Text>
          )}
        </Box>

        <Divider />

        {canViewStockNotifications ? (
          <Tabs variant="enclosed" size="sm">
            <TabList>
              <Tab>
                {t('notifications.tabs.approval')}
                {approvalUnreadCount > 0 && (
                  <Badge ml={2} colorScheme="blue" borderRadius="full" fontSize="0.6em">
                    {approvalUnreadCount}
                  </Badge>
                )}
              </Tab>
              <Tab>
                {t('notifications.tabs.stock')}
                {(stockUnreadCount + stockAlerts.length) > 0 && (
                  <Badge ml={2} colorScheme="red" borderRadius="full" fontSize="0.6em">
                    {stockUnreadCount + stockAlerts.length}
                  </Badge>
                )}
              </Tab>
            </TabList>

            <TabPanels>
              {/* Approval Notifications Tab */}
              <TabPanel p={0}>
                <Box maxH="300px" overflowY="auto">
                  {approvalNotifications.length === 0 ? (
                    <MenuItem isDisabled>
                      <Text fontSize="sm" color="gray.500">{t('notifications.empty.approval')}</Text>
                    </MenuItem>
                  ) : (
                    approvalNotifications.map((n) => (
                      <MenuItem
                        key={n.id}
                        onClick={() => {
                          if (!n.is_read) handleMarkApprovalAsRead(n.id);
                        }}
                      >
                        <HStack align="start" spacing={3} w="full">
                          <Box pt={1}>{getApprovalIcon(n.type)}</Box>
                          <VStack align="start" spacing={0} flex={1}>
                            <HStack justify="space-between" w="full">
                              <Text fontSize="sm" fontWeight={n.is_read ? 'normal' : 'semibold'}>
                                {n.title}
                              </Text>
                              {!n.is_read && <Box w={2} h={2} bg="blue.500" borderRadius="full" />}
                            </HStack>
                            <Text fontSize="sm" color="gray.600" noOfLines={2}>{formatMessageCurrency(n.message, n.data)}</Text>
                            <HStack spacing={2} pt={1}>
                              <Text fontSize="xs" color="gray.500">{formatDate(n.created_at)}</Text>
                              <Badge colorScheme="gray" fontSize="0.6em">{n.priority}</Badge>
                            </HStack>
                          </VStack>
                        </HStack>
                      </MenuItem>
                    ))
                  )}
                </Box>
              </TabPanel>

              {/* Stock Notifications Tab */}
              <TabPanel p={0}>
                <Box maxH="300px" overflowY="auto">
                  {stockAlerts.length === 0 && stockNotifications.length === 0 ? (
                    <MenuItem isDisabled>
                      <Text fontSize="sm" color="gray.500">{t('notifications.empty.stock')}</Text>
                    </MenuItem>
                  ) : (
                    <>
                      {/* Active Stock Alerts */}
                      {stockAlerts.map((alert) => (
                        <Box key={`alert-${alert.id}`} p={3} borderBottom="1px" borderColor="gray.200">
                          <HStack align="start" spacing={3}>
                            <Box pt={1}>{getStockIcon(alert.urgency)}</Box>
                            <VStack align="start" spacing={1} flex={1}>
                              <HStack justify="space-between" w="full">
                                <Text fontSize="sm" fontWeight="bold">
                                  {alert.product_name}
                                </Text>
                                <Badge colorScheme={getUrgencyColor(alert.urgency)} fontSize="0.6em">
                                  {alert.urgency}
                                </Badge>
                              </HStack>

                              <Text fontSize="xs" color="gray.600">
                                {t('notifications.stock.code')}: {alert.product_code} {alert.category_name && `• ${alert.category_name}`}
                              </Text>

                              <Alert status="warning" size="sm" borderRadius="md">
                                <AlertIcon />
                                <Text fontSize="xs">
                                  {t('notifications.stock.current')}: {alert.current_stock} / {t('notifications.stock.min')}: {alert.threshold_stock}
                                </Text>
                              </Alert>

                              <Button
                                size="xs"
                                colorScheme="gray"
                                onClick={() => dismissStockAlert(alert.id)}
                              >
                                {t('notifications.stock.dismiss')}
                              </Button>
                            </VStack>
                          </HStack>
                        </Box>
                      ))}

                      {/* Stock Notifications */}
                      {stockNotifications.map((notif) => {
                        const data = typeof notif.data === 'string' ? JSON.parse(notif.data) : notif.data;
                        return (
                          <MenuItem
                            key={`notif-${notif.id}`}
                            onClick={() => {
                              if (!notif.is_read) handleMarkStockAsRead(notif.id);
                            }}
                          >
                            <HStack align="start" spacing={3} w="full">
                              <Box pt={1}>
                                <FiAlertTriangle color="#dd6b20" />
                              </Box>
                              <VStack align="start" spacing={0} flex={1}>
                                <HStack justify="space-between" w="full">
                                  <Text fontSize="sm" fontWeight={notif.is_read ? 'normal' : 'semibold'}>
                                    {notif.title}
                                  </Text>
                                  {!notif.is_read && <Box w={2} h={2} bg="orange.500" borderRadius="full" />}
                                </HStack>

                                <Text fontSize="xs" color="gray.600" noOfLines={2}>
                                  {formatMessageCurrency(notif.message, notif.data)}
                                </Text>

                                {data && (
                                  <Text fontSize="xs" color="gray.500">
                                    {t('notifications.stock.current')}: {data.current_stock} / {t('notifications.stock.min')}: {data.minimum_stock}
                                  </Text>
                                )}

                                <Text fontSize="xs" color="gray.500">
                                  {formatDate(notif.created_at)}
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
        ) : (
          // Show only approval notifications for non-admin users
          <Box maxH="350px" overflowY="auto">
            {approvalNotifications.length === 0 ? (
              <MenuItem isDisabled>
                <Text fontSize="sm" color="gray.500">{t('notifications.empty.general')}</Text>
              </MenuItem>
            ) : (
              approvalNotifications.map((n) => (
                <MenuItem
                  key={n.id}
                  onClick={() => {
                    if (!n.is_read) handleMarkApprovalAsRead(n.id);
                  }}
                >
                  <HStack align="start" spacing={3} w="full">
                    <Box pt={1}>{getApprovalIcon(n.type)}</Box>
                    <VStack align="start" spacing={0} flex={1}>
                      <HStack justify="space-between" w="full">
                        <Text fontSize="sm" fontWeight={n.is_read ? 'normal' : 'semibold'}>
                          {n.title}
                        </Text>
                        {!n.is_read && <Box w={2} h={2} bg="blue.500" borderRadius="full" />}
                      </HStack>
                      <Text fontSize="sm" color="gray.600" noOfLines={2}>{formatMessageCurrency(n.message, n.data)}</Text>
                      <HStack spacing={2} pt={1}>
                        <Text fontSize="xs" color="gray.500">{formatDate(n.created_at)}</Text>
                        <Badge colorScheme="gray" fontSize="0.6em">{n.priority}</Badge>
                      </HStack>
                    </VStack>
                  </HStack>
                </MenuItem>
              ))
            )}
          </Box>
        )}
      </MenuList>

      {/* Authentication Error Alert Dialog */}
      <AlertDialog
        isOpen={isAuthErrorOpen}
        leastDestructiveRef={cancelRef}
        onClose={handleCloseAuthError}
        isCentered
      >
        <AlertDialogOverlay>
          <AlertDialogContent>
            <AlertDialogHeader fontSize="lg" fontWeight="bold">
              🔐 {t('auth.session.expiredTitle')}
            </AlertDialogHeader>

            <AlertDialogBody>
              <VStack align="start" spacing={3}>
                <Text>
                  {t('auth.session.expiredMessage')}
                </Text>
                <Alert status="warning" borderRadius="md">
                  <AlertIcon />
                  <Box flex="1">
                    <Text fontSize="sm">
                      {t('auth.session.securityNote')}
                    </Text>
                  </Box>
                </Alert>
                <Text fontSize="sm" color="gray.600">
                  {t('auth.session.possibleCauses')}
                </Text>
                <VStack align="start" spacing={1} pl={4} fontSize="sm" color="gray.600">
                  <Text>{t('auth.session.tokenExpired')}</Text>
                  <Text>{t('auth.session.otherDevice')}</Text>
                  <Text>{t('auth.session.timeout')}</Text>
                </VStack>
              </VStack>
            </AlertDialogBody>

            <AlertDialogFooter>
              <Button ref={cancelRef} onClick={handleCloseAuthError} variant="ghost">
                {t('auth.session.close')}
              </Button>
              <Button
                colorScheme="blue"
                onClick={handleLogout}
                ml={3}
                leftIcon={<FiLogOut />}
              >
                {t('auth.session.relogin')}
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialogOverlay>
      </AlertDialog>
    </Menu>
  );
};

export default UnifiedNotifications;

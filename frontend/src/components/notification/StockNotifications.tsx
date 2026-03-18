'use client';

import React, { useEffect, useState } from 'react';
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
  Spinner,
  Text,
  VStack,
} from '@chakra-ui/react';
import { FiAlertCircle, FiAlertTriangle, FiPackage } from 'react-icons/fi';
import axios from 'axios';
import { API_BASE_URL } from '@/config/api';
import { useAuth } from '@/contexts/AuthContext';

interface StockNotification {
  id: number;
  title: string;
  message: string;
  is_read: boolean;
  created_at: string;
  data?: unknown;
}

interface StockAlert {
  id: number;
  product_name: string;
  product_code: string;
  current_stock: number;
  threshold_stock: number;
  urgency: string;
  category_name?: string;
}

const StockNotifications: React.FC = () => {
  const { user } = useAuth();
  const [notifications, setNotifications] = useState<StockNotification[]>([]);
  const [stockAlerts, setStockAlerts] = useState<StockAlert[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [loading, setLoading] = useState(false);

  const canViewStockNotifications =
    user?.role === 'admin' ||
    user?.role === 'inventory_manager' ||
    user?.role === 'director';
  const canDismissStockAlerts =
    user?.role === 'admin' ||
    user?.role === 'inventory_manager';

  useEffect(() => {
    if (!canViewStockNotifications) {
      return;
    }

    const refresh = () => {
      void refreshStockData();
    };

    refresh();
    const interval = setInterval(() => {
      refresh();
    }, 300000);

    return () => clearInterval(interval);
    // Legacy component refresh only follows role visibility changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [canViewStockNotifications]);

  const refreshStockData = async () => {
    await Promise.all([fetchStockNotifications(), fetchStockAlerts()]);
  };

  const fetchStockNotifications = async () => {
    try {
      setLoading(true);
      const token = localStorage.getItem('token');
      const [listResponse, unreadCountResponse] = await Promise.all([
        axios.get(`${API_BASE_URL}/api/v1/notifications`, {
          params: { types: 'LOW_STOCK,REORDER_ALERT,STOCK_OUT', limit: 20 },
          headers: { Authorization: `Bearer ${token}` }
        }),
        axios.get(`${API_BASE_URL}/api/v1/notifications/summary`, {
          headers: { Authorization: `Bearer ${token}` }
        }),
      ]);

      const allNotifications = (listResponse.data.notifications || [])
        .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
        .slice(0, 10);

      const summary = unreadCountResponse.data.summary || {};
      setNotifications(allNotifications);
      setUnreadCount(summary.stock_unread || 0);
    } catch (error) {
      console.error('Failed to fetch stock notifications:', error);
      setNotifications([]);
      setUnreadCount(0);
    } finally {
      setLoading(false);
    }
  };

  const fetchStockAlerts = async () => {
    try {
      const token = localStorage.getItem('token');
      const response = await axios.get(`${API_BASE_URL}/api/v1/dashboard/stock-alerts`, {
        headers: { Authorization: `Bearer ${token}` }
      });

      if (response.data.data?.alerts) {
        setStockAlerts(response.data.data.alerts);
      } else {
        setStockAlerts(response.data.alerts || []);
      }
    } catch (error) {
      console.error('Failed to fetch stock alerts:', error);
      setStockAlerts([]);
    }
  };

  const handleMarkAsRead = async (notificationId: number) => {
    try {
      const token = localStorage.getItem('token');
      await axios.put(
        `${API_BASE_URL}/api/v1/notifications/${notificationId}/read`,
        {},
        { headers: { Authorization: `Bearer ${token}` } }
      );

      setNotifications((prev) =>
        prev.map((notification) =>
          notification.id === notificationId
            ? { ...notification, is_read: true }
            : notification
        )
      );
      setUnreadCount((prev) => Math.max(0, prev - 1));
    } catch (error) {
      console.error('Failed to mark notification as read:', error);
    }
  };

  const dismissAlert = async (alertId: number) => {
    try {
      const token = localStorage.getItem('token');
      await axios.post(
        `${API_BASE_URL}/api/v1/dashboard/stock-alerts/${alertId}/dismiss`,
        {},
        { headers: { Authorization: `Bearer ${token}` } }
      );

      setStockAlerts((prev) => prev.filter((alert) => alert.id !== alertId));
    } catch (error) {
      console.error('Failed to dismiss alert:', error);
    }
  };

  const getIcon = (urgency: string) => {
    switch (urgency) {
      case 'critical':
        return <FiAlertTriangle color="#e53e3e" />;
      case 'high':
        return <FiAlertCircle color="#dd6b20" />;
      default:
        return <FiPackage color="#3182ce" />;
    }
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

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    const now = new Date();
    const diffInHours = (now.getTime() - date.getTime()) / (1000 * 60 * 60);

    if (diffInHours < 1) return 'Baru saja';
    if (diffInHours < 24) return `${Math.floor(diffInHours)} jam lalu`;
    return date.toLocaleDateString('id-ID', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  };

  const parseNotificationData = (data?: unknown) => {
    if (!data) return null;
    if (typeof data !== 'string') return data as Record<string, unknown>;

    try {
      return JSON.parse(data) as Record<string, unknown>;
    } catch {
      return null;
    }
  };

  if (!canViewStockNotifications) {
    return null;
  }

  return (
    <Menu placement="bottom-end" autoSelect={false} onOpen={refreshStockData}>
      <MenuButton as={IconButton} aria-label="Stock Notifications" variant="ghost">
        <Box position="relative">
          <FiPackage />
          {unreadCount > 0 && (
            <Badge
              colorScheme="red"
              borderRadius="full"
              position="absolute"
              top={-2}
              right={-2}
              fontSize="0.6em"
              px={2}
            >
              {unreadCount}
            </Badge>
          )}
        </Box>
      </MenuButton>

      <MenuList minW="400px" maxW="90vw">
        <Box px={3} pt={2} pb={1}>
          <Text fontSize="md" fontWeight="bold">Stock Alerts</Text>
          {(stockAlerts.length > 0 || unreadCount > 0) && (
            <Text fontSize="xs" color="gray.500">
              {stockAlerts.length} active alert{stockAlerts.length !== 1 ? 's' : ''}, {unreadCount} unread
            </Text>
          )}
        </Box>

        <Divider />

        {loading ? (
          <MenuItem isDisabled>
            <HStack spacing={2}>
              <Spinner size="xs" />
              <Text fontSize="sm" color="gray.500">Loading stock alerts...</Text>
            </HStack>
          </MenuItem>
        ) : stockAlerts.length === 0 && notifications.length === 0 ? (
          <MenuItem isDisabled>
            <Text fontSize="sm" color="gray.500">No stock alerts</Text>
          </MenuItem>
        ) : (
          <Box maxH="400px" overflowY="auto">
            {stockAlerts.map((alert) => (
              <Box key={`alert-${alert.id}`} p={3} borderBottom="1px" borderColor="gray.200">
                <HStack align="start" spacing={3}>
                  <Box pt={1}>{getIcon(alert.urgency)}</Box>
                  <VStack align="start" spacing={1} flex={1}>
                    <HStack justify="space-between" w="full">
                      <Text fontSize="sm" fontWeight="bold">
                        {alert.product_name}
                      </Text>
                      <Badge colorScheme={getUrgencyColor(alert.urgency)}>
                        {alert.urgency}
                      </Badge>
                    </HStack>

                    <Text fontSize="xs" color="gray.600">
                      Code: {alert.product_code}{alert.category_name ? ` - ${alert.category_name}` : ''}
                    </Text>

                    <Alert status="warning" size="sm" borderRadius="md">
                      <AlertIcon />
                      <Text fontSize="xs">
                        Stock: {alert.current_stock} / Min: {alert.threshold_stock}
                      </Text>
                    </Alert>

                    {canDismissStockAlerts && (
                      <Button
                        size="xs"
                        colorScheme="gray"
                        onClick={() => dismissAlert(alert.id)}
                      >
                        Dismiss
                      </Button>
                    )}
                  </VStack>
                </HStack>
              </Box>
            ))}

            {notifications.map((notification) => {
              const data = parseNotificationData(notification.data);
              return (
                <MenuItem
                  key={`notif-${notification.id}`}
                  onClick={() => {
                    if (!notification.is_read) {
                      handleMarkAsRead(notification.id);
                    }
                  }}
                >
                  <HStack align="start" spacing={3} w="full">
                    <Box pt={1}>
                      <FiAlertTriangle color="#dd6b20" />
                    </Box>
                    <VStack align="start" spacing={0} flex={1}>
                      <HStack justify="space-between" w="full">
                        <Text fontSize="sm" fontWeight={notification.is_read ? 'normal' : 'semibold'}>
                          {notification.title}
                        </Text>
                        {!notification.is_read && <Box w={2} h={2} bg="orange.500" borderRadius="full" />}
                      </HStack>

                      <Text fontSize="xs" color="gray.600" noOfLines={2}>
                        {notification.message}
                      </Text>

                      {data && (
                        <Text fontSize="xs" color="gray.500">
                          Stock: {String(data.current_stock ?? '-')} / Min: {String(data.minimum_stock ?? data.reorder_level ?? '-')}
                        </Text>
                      )}

                      <Text fontSize="xs" color="gray.500">
                        {formatDate(notification.created_at)}
                      </Text>
                    </VStack>
                  </HStack>
                </MenuItem>
              );
            })}
          </Box>
        )}

        {(stockAlerts.length > 0 || notifications.length > 0) && (
          <>
            <Divider />
            <MenuItem onClick={() => { window.location.href = '/inventory'; }}>
              <Text w="full" textAlign="center" fontSize="sm" color="blue.600">
                Go to Inventory
              </Text>
            </MenuItem>
          </>
        )}
      </MenuList>
    </Menu>
  );
};

export default StockNotifications;

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
} from '@chakra-ui/react';
import { FiBell, FiPackage, FiAlertTriangle, FiAlertCircle } from 'react-icons/fi';
import axios from 'axios';
import { API_BASE_URL } from '@/config/api';
import { useAuth } from '@/contexts/AuthContext';

interface StockNotification {
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

const StockNotifications: React.FC = () => {
  const [notifications, setNotifications] = useState<StockNotification[]>([]);
  const [stockAlerts, setStockAlerts] = useState<StockAlert[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [loading, setLoading] = useState(false);
  const { user } = useAuth();

  useEffect(() => {
    if (user?.role === 'admin' || user?.role === 'inventory_manager') {
      fetchStockNotifications();
      fetchStockAlerts();
      // Increase interval to 5 minutes to reduce server load
      const interval = setInterval(() => {
        fetchStockNotifications();
        fetchStockAlerts();
      }, 300000); // Refresh every 5 minutes (300 seconds)
      return () => clearInterval(interval);
    }
  }, [user]);

  const fetchStockNotifications = async () => {
    try {
      setLoading(true);
      const token = localStorage.getItem('token');

      // Get MIN_STOCK, REORDER_ALERT and STOCK_OUT notifications
      const [minStockResponse, reorderResponse, stockOutResponse] = await Promise.all([
        axios.get(`${API_BASE_URL}/api/v1/notifications`, {
          params: { type: 'MIN_STOCK', limit: 10 },
          headers: { Authorization: `Bearer ${token}` }
        }),
        axios.get(`${API_BASE_URL}/api/v1/notifications`, {
          params: { type: 'REORDER_ALERT', limit: 10 },
          headers: { Authorization: `Bearer ${token}` }
        }),
        axios.get(`${API_BASE_URL}/api/v1/notifications`, {
          params: { type: 'STOCK_OUT', limit: 10 },
          headers: { Authorization: `Bearer ${token}` }
        })
      ]);

      const minStockNotifs = minStockResponse.data.notifications || [];
      const reorderNotifs = reorderResponse.data.notifications || [];
      const stockOutNotifs = stockOutResponse.data.notifications || [];

      const allNotifs = [...minStockNotifs, ...reorderNotifs, ...stockOutNotifs]
        .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
        .slice(0, 10);

      setNotifications(allNotifs);

      // Get unread count for all types
      const [minStockCount, reorderCount, stockOutCount] = await Promise.all([
        axios.get(`${API_BASE_URL}/api/v1/notifications/unread-count`, {
          params: { type: 'MIN_STOCK' },
          headers: { Authorization: `Bearer ${token}` }
        }),
        axios.get(`${API_BASE_URL}/api/v1/notifications/unread-count`, {
          params: { type: 'REORDER_ALERT' },
          headers: { Authorization: `Bearer ${token}` }
        }),
        axios.get(`${API_BASE_URL}/api/v1/notifications/unread-count`, {
          params: { type: 'STOCK_OUT' },
          headers: { Authorization: `Bearer ${token}` }
        })
      ]);

      const totalUnread = (minStockCount.data.count || 0) +
        (reorderCount.data.count || 0) +
        (stockOutCount.data.count || 0);
      setUnreadCount(totalUnread);
    } catch (error) {
      console.error('Failed to fetch stock notifications:', error);
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
      }
    } catch (error) {
      console.error('Failed to fetch stock alerts:', error);
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

      setNotifications(prev =>
        prev.map(n => n.id === notificationId ? { ...n, is_read: true } : n)
      );
      setUnreadCount(prev => Math.max(0, prev - 1));
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

      setStockAlerts(prev => prev.filter(a => a.id !== alertId));
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
      case 'critical': return 'red';
      case 'high': return 'orange';
      case 'medium': return 'yellow';
      default: return 'blue';
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

  // Don't show for non-authorized roles
  if (user?.role !== 'admin' && user?.role !== 'inventory_manager') {
    return null;
  }

  const totalAlerts = stockAlerts.length + unreadCount;

  return (
    <Menu placement="bottom-end" autoSelect={false} onOpen={fetchStockNotifications}>
      <MenuButton as={IconButton} aria-label="Stock Notifications" variant="ghost">
        <Box position="relative">
          <FiPackage />
          {totalAlerts > 0 && (
            <Badge
              colorScheme="red"
              borderRadius="full"
              position="absolute"
              top={-2}
              right={-2}
              fontSize="0.6em"
              px={2}
            >
              {totalAlerts}
            </Badge>
          )}
        </Box>
      </MenuButton>

      <MenuList minW="400px" maxW="90vw">
        <Box px={3} pt={2} pb={1}>
          <Text fontSize="md" fontWeight="bold">Stock Alerts</Text>
          {totalAlerts > 0 && (
            <Text fontSize="xs" color="gray.500">
              {stockAlerts.length} active alert{stockAlerts.length !== 1 ? 's' : ''},
              {unreadCount} unread
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
            {/* Active Stock Alerts */}
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
                      Code: {alert.product_code} {alert.category_name && `• ${alert.category_name}`}
                    </Text>

                    <Alert status="warning" size="sm" borderRadius="md">
                      <AlertIcon />
                      <Text fontSize="xs">
                        Stock: {alert.current_stock} / Min: {alert.threshold_stock}
                      </Text>
                    </Alert>

                    <Button
                      size="xs"
                      colorScheme="gray"
                      onClick={() => dismissAlert(alert.id)}
                    >
                      Dismiss
                    </Button>
                  </VStack>
                </HStack>
              </Box>
            ))}

            {/* Notifications */}
            {notifications.map((notif) => {
              const data = typeof notif.data === 'string' ? JSON.parse(notif.data) : notif.data;
              return (
                <MenuItem
                  key={`notif-${notif.id}`}
                  onClick={() => {
                    if (!notif.is_read) handleMarkAsRead(notif.id);
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
                        {notif.message}
                      </Text>

                      {data && (
                        <Text fontSize="xs" color="gray.500">
                          Stock: {data.current_stock} / Min: {data.minimum_stock}
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
          </Box>
        )}

        {(stockAlerts.length > 0 || notifications.length > 0) && (
          <>
            <Divider />
            <MenuItem
              onClick={() => window.location.href = '/inventory'}
            >
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

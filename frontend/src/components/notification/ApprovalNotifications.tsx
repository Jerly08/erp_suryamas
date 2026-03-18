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
} from '@chakra-ui/react';
import { FiBell, FiCheckCircle, FiXCircle, FiClock, FiShoppingCart } from 'react-icons/fi';
import approvalService from '../../services/approvalService';
import { formatIDR } from '@/utils/currency';

interface NotificationItem {
  id: number;
  type: string;
  title: string;
  message: string;
  priority: string;
  is_read: boolean;
  created_at: string;
  data?: string;
}

const APPROVAL_NOTIFICATION_TYPES = [
  'APPROVAL_PENDING',
  'APPROVAL_APPROVED',
  'APPROVAL_REJECTED',
];

const ApprovalNotifications: React.FC = () => {
  const [notifications, setNotifications] = useState<NotificationItem[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetchNotifications();
    fetchUnreadCount();
    const interval = setInterval(() => fetchUnreadCount(), 30000);
    return () => clearInterval(interval);
  }, []);

  const fetchNotifications = async () => {
    try {
      setLoading(true);
      const response = await approvalService.getNotifications({ limit: 20, type: 'approval' });
      setNotifications(response.notifications || []);
    } catch (error: unknown) {
      // Handle authentication errors gracefully
      if (approvalServiceErrorStatus(error) === 401) {
        console.log('Authentication required for notifications');
        setNotifications([]);
        return;
      }
      
      // Handle other errors
      if (approvalServiceErrorStatus(error) !== 403 && !approvalServiceErrorMessage(error).includes('AUTH_SESSION_EXPIRED')) {
        console.error('Failed to fetch notifications:', error);
      }
      
      setNotifications([]);
    } finally {
      setLoading(false);
    }
  };

  const fetchUnreadCount = async () => {
    try {
      const response = await approvalService.getUnreadNotificationCount({ types: APPROVAL_NOTIFICATION_TYPES });
      setUnreadCount(response.count || 0);
    } catch (error: unknown) {
      // Handle authentication errors gracefully
      if (approvalServiceErrorStatus(error) === 401) {
        console.log('Authentication required for notifications, count reset to 0');
        setUnreadCount(0);
        return;
      }
      
      // Handle other errors but don't spam the console
      if (approvalServiceErrorStatus(error) !== 403 && !approvalServiceErrorMessage(error).includes('AUTH_SESSION_EXPIRED')) {
        console.error('Failed to fetch unread count:', error);
      }
      
      // Gracefully handle by setting count to 0
      setUnreadCount(0);
    }
  };

  const handleMarkAsRead = async (notificationId: number) => {
    try {
      await approvalService.markNotificationAsRead(notificationId);
      setNotifications(prev => prev.map(n => (n.id === notificationId ? { ...n, is_read: true } : n)));
      fetchUnreadCount();
    } catch (error) {
      console.error('Failed to mark notification as read:', error);
    }
  };

  const getIcon = (type: string) => {
    switch (type) {
      case 'approval_pending':
        return <FiClock color="#dd6b20" />; // orange.400
      case 'approval_approved':
        return <FiCheckCircle color="#38a169" />; // green.500
      case 'approval_rejected':
        return <FiXCircle color="#e53e3e" />; // red.500
      default:
        return <FiShoppingCart color="#3182ce" />; // blue.600
    }
  };

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    const now = new Date();
    const diffInHours = (now.getTime() - date.getTime()) / (1000 * 60 * 60);
    if (diffInHours < 1) return 'Just now';
    if (diffInHours < 24) return `${Math.floor(diffInHours)}h ago`;
    return date.toLocaleDateString('id-ID', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
  };

  const formatMessageCurrency = (message: string, data?: unknown) => {
    if (!message) return '';

    let result = message;

    // If data contains total_amount, update inline amount
    const parsed = (() => {
      if (!data) return null;
      try {
        return typeof data === 'string' ? JSON.parse(data) : data;
      } catch {
        return null;
      }
    })();

    const totalAmount = parsed?.total_amount ?? parsed?.amount ?? parsed?.totalAmount;
    if (typeof totalAmount === 'number') {
      result = result.replace(/\(\s*Amount\s*:\s*[^\)]*\)/i, () => `(${'Amount'}: ${formatIDR(totalAmount)})`);
    }

    result = result.replace(/(Amount|amount)\s*:\s*(\d+[\d.,]*)/g, (_m, lbl, num) => {
      const normalized = parseFloat(String(num).replace(/\./g, '').replace(/,/g, '.'));
      const formatted = isNaN(normalized) ? num : formatIDR(normalized);
      return `${lbl}: ${formatted}`;
    });

    return result;
  };

  const priorityColor = (priority: string) => {
    const p = (priority || '').toLowerCase();
    if (p === 'high' || p === 'urgent') return 'red.500';
    if (p === 'normal') return 'orange.400';
    if (p === 'low') return 'blue.500';
    return 'gray.500';
  };

  return (
    <Menu placement="bottom-end" autoSelect={false} onOpen={fetchNotifications}>
      <MenuButton as={IconButton} aria-label="Notifications" variant="ghost">
        <Box position="relative">
          <FiBell />
          {unreadCount > 0 && (
            <Badge colorScheme="red" borderRadius="full" position="absolute" top={-2} right={-2} fontSize="0.6em" px={2}>
              {unreadCount}
            </Badge>
          )}
        </Box>
      </MenuButton>
      <MenuList minW="380px" maxW="90vw">
        <Box px={3} pt={2} pb={1}>
          <Text fontSize="md" fontWeight="bold">Notifications</Text>
          {unreadCount > 0 && (
            <Text fontSize="xs" color="gray.500">{unreadCount} unread notification{unreadCount > 1 ? 's' : ''}</Text>
          )}
        </Box>
        <Divider />
        {loading ? (
          <MenuItem isDisabled>
            <HStack spacing={2}>
              <Spinner size="xs" />
              <Text fontSize="sm" color="gray.500">Loading notifications...</Text>
            </HStack>
          </MenuItem>
        ) : notifications.length === 0 ? (
          <MenuItem isDisabled>
            <Text fontSize="sm" color="gray.500">No notifications</Text>
          </MenuItem>
        ) : (
          <Box maxH="350px" overflowY="auto">
            {notifications.map((n) => (
              <Box key={n.id}>
                <MenuItem
                  onClick={() => {
                    if (!n.is_read) handleMarkAsRead(n.id);
                  }}
                >
                  <HStack align="start" spacing={3} w="full">
                    <Box pt={1}>{getIcon(n.type)}</Box>
                    <VStack align="start" spacing={0} flex={1}>
                      <HStack justify="space-between" w="full">
                        <Text fontSize="sm" fontWeight={n.is_read ? 'normal' : 'semibold'}>{n.title}</Text>
                        {!n.is_read && <Box w={2} h={2} bg="blue.500" borderRadius="full" />}
                      </HStack>
                      <Text fontSize="sm" color="gray.600" noOfLines={2}>{formatMessageCurrency(n.message, n.data)}</Text>
                      <HStack spacing={2} pt={1}>
                        <Text fontSize="xs" color="gray.500">{formatDate(n.created_at)}</Text>
                        <Badge colorScheme="gray" bg={priorityColor(n.priority)} color="white">{n.priority}</Badge>
                      </HStack>
                    </VStack>
                  </HStack>
                </MenuItem>
                <Divider />
              </Box>
            ))}
          </Box>
        )}
        {notifications.length > 0 && (
          <>
            <Divider />
            <MenuItem onClick={() => { /* future navigation */ }}>
              <Text w="full" textAlign="center" fontSize="sm" color="blue.600">View All Notifications</Text>
            </MenuItem>
          </>
        )}
      </MenuList>
    </Menu>
  );
};

const approvalServiceErrorStatus = (error: unknown) => {
  if (typeof error === 'object' && error !== null && 'response' in error) {
    const response = (error as { response?: { status?: number } }).response;
    return response?.status;
  }
  return undefined;
};

const approvalServiceErrorMessage = (error: unknown) => {
  if (typeof error === 'object' && error !== null && 'message' in error) {
    return String((error as { message?: unknown }).message ?? '');
  }
  return '';
};

export default ApprovalNotifications;

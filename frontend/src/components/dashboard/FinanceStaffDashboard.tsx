"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import api from "@/services/api";
import { API_ENDPOINTS } from "@/config/api";
import {
  Alert,
  AlertIcon,
  Badge,
  Box,
  Button,
  Card,
  CardBody,
  CardHeader,
  Flex,
  Grid,
  GridItem,
  Heading,
  HStack,
  Icon,
  SimpleGrid,
  Spinner,
  Text,
  VStack,
  useColorModeValue,
} from "@chakra-ui/react";
import {
  FiBell,
  FiClipboard,
  FiClock,
  FiDollarSign,
  FiFileText,
  FiRepeat,
} from "react-icons/fi";

interface FinanceStaffQuickStats {
  pending_purchase_requests_count: number;
  draft_journals_count: number;
  unpaid_purchase_count: number;
  pending_transfer_requests_count: number;
  monthly_reviewed_purchase_count: number;
  unread_notifications_count: number;
}

interface FinanceStaffSnapshot {
  outstanding_receivables: number;
  outstanding_payables: number;
  cash_bank_balance: number;
}

interface PendingPurchaseRequestItem {
  id: number;
  pr_number: string;
  requester_name: string;
  department?: string;
  project_name?: string;
  total_estimated_amount: number;
  created_at: string;
}

interface RecentTransferRequestItem {
  id: number;
  transfer_number: string;
  date: string;
  amount: number;
  converted_amount: number;
  status: string;
  reference?: string;
  rejection_reason?: string;
  from_account_name?: string;
  from_currency?: string;
  to_account_name?: string;
  to_currency?: string;
}

interface RecentNotificationItem {
  id: number;
  type: string;
  title: string;
  message: string;
  priority: string;
  is_read: boolean;
  created_at: string;
}

interface FinanceStaffDashboardData {
  quick_stats?: FinanceStaffQuickStats;
  snapshot?: FinanceStaffSnapshot;
  pending_purchase_requests?: PendingPurchaseRequestItem[];
  recent_transfer_requests?: RecentTransferRequestItem[];
  recent_notifications?: RecentNotificationItem[];
}

interface ApiErrorShape {
  response?: {
    data?: {
      error?: string;
    };
  };
  message?: string;
}

interface FinanceStaffStatCardProps {
  title: string;
  value: string | number;
  helper: string;
  icon: React.ElementType;
  accentColor: string;
}

const FinanceStaffStatCard: React.FC<FinanceStaffStatCardProps> = ({
  title,
  value,
  helper,
  icon,
  accentColor,
}) => {
  const cardBg = useColorModeValue("white", "gray.800");
  const borderColor = useColorModeValue("gray.200", "whiteAlpha.200");
  const iconBg = useColorModeValue(`${accentColor}.50`, `${accentColor}.900`);
  const mutedTextColor = useColorModeValue("gray.500", "whiteAlpha.700");

  return (
    <Card
      bg={cardBg}
      border="1px"
      borderColor={borderColor}
      boxShadow="sm"
      h="100%"
    >
      <CardBody>
        <Flex justify="space-between" align="start" gap={4}>
          <Box>
            <Text fontSize="sm" color={mutedTextColor} mb={1}>
              {title}
            </Text>
            <Heading size="lg">{value}</Heading>
            <Text fontSize="sm" color={mutedTextColor} mt={2}>
              {helper}
            </Text>
          </Box>
          <Flex
            align="center"
            justify="center"
            w={11}
            h={11}
            borderRadius="xl"
            bg={iconBg}
            color={`${accentColor}.500`}
            flexShrink={0}
          >
            <Icon as={icon} boxSize={5} />
          </Flex>
        </Flex>
      </CardBody>
    </Card>
  );
};

const getTransferStatusColor = (status?: string): string => {
  switch ((status || "").toUpperCase()) {
    case "PENDING_APPROVAL":
      return "orange";
    case "COMPLETED":
      return "green";
    case "REJECTED":
      return "red";
    default:
      return "gray";
  }
};

const getTransferStatusLabel = (status?: string): string => {
  switch ((status || "").toUpperCase()) {
    case "PENDING_APPROVAL":
      return "Waiting Manager";
    case "COMPLETED":
      return "Approved";
    case "REJECTED":
      return "Rejected";
    default:
      return status || "-";
  }
};

const getNotificationColor = (priority?: string): string => {
  switch ((priority || "").toUpperCase()) {
    case "URGENT":
      return "red";
    case "HIGH":
      return "orange";
    case "MEDIUM":
      return "blue";
    default:
      return "gray";
  }
};

const formatCurrency = (amount: number): string =>
  new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(amount || 0);

const formatTransferAmount = (transfer: RecentTransferRequestItem): string => {
  const sourceCurrency = (transfer.from_currency || "IDR").toUpperCase();
  const destinationCurrency = (
    transfer.to_currency || sourceCurrency
  ).toUpperCase();
  const sourceAmount = `${sourceCurrency} ${(transfer.amount || 0).toLocaleString("id-ID")}`;

  if (sourceCurrency !== destinationCurrency) {
    return `${sourceAmount} -> ${destinationCurrency} ${(transfer.converted_amount || 0).toLocaleString("id-ID")}`;
  }

  return sourceAmount;
};

export const FinanceStaffDashboard: React.FC = () => {
  const router = useRouter();
  const [data, setData] = useState<FinanceStaffDashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const headingColor = useColorModeValue("gray.800", "whiteAlpha.900");
  const mutedTextColor = useColorModeValue("gray.500", "whiteAlpha.700");
  const cardBg = useColorModeValue("white", "gray.800");
  const cardBorderColor = useColorModeValue("gray.200", "whiteAlpha.200");
  const softPanelBg = useColorModeValue("gray.50", "whiteAlpha.50");

  useEffect(() => {
    const fetchDashboard = async () => {
      try {
        setLoading(true);
        const response = await api.get(API_ENDPOINTS.DASHBOARD_FINANCE_STAFF);
        setData(response.data?.data || response.data || {});
        setError(null);
      } catch (fetchError: unknown) {
        console.error(
          "Error fetching finance staff dashboard data:",
          fetchError,
        );
        const normalizedError = fetchError as ApiErrorShape;
        setError(
          normalizedError.response?.data?.error ||
            normalizedError.message ||
            "Failed to load finance staff dashboard",
        );
      } finally {
        setLoading(false);
      }
    };

    void fetchDashboard();
  }, []);

  if (loading) {
    return (
      <Flex justify="center" align="center" minH="50vh">
        <VStack spacing={4}>
          <Spinner size="xl" color="blue.500" thickness="4px" />
          <Text>Memuat dashboard finance staff...</Text>
        </VStack>
      </Flex>
    );
  }

  if (error) {
    return (
      <Box>
        <Heading as="h2" size="xl" mb={6} color={headingColor}>
          Dashboard - Finance Staff
        </Heading>
        <Alert status="error" borderRadius="md">
          <AlertIcon />
          {error}
        </Alert>
      </Box>
    );
  }

  const quickStats = data?.quick_stats;
  const snapshot = data?.snapshot;
  const pendingPurchaseRequests = data?.pending_purchase_requests || [];
  const recentTransferRequests = data?.recent_transfer_requests || [];
  const recentNotifications = data?.recent_notifications || [];

  return (
    <Box>
      <Heading as="h2" size="xl" mb={2} color={headingColor}>
        Dashboard - Finance Staff
      </Heading>
      <Text color={mutedTextColor} mb={6}>
        Operational view for purchase request review, journal preparation,
        payment follow-up, and transfer requests waiting finance manager
        approval.
      </Text>

      <SimpleGrid columns={{ base: 1, md: 2, xl: 4 }} spacing={4} mb={6}>
        <FinanceStaffStatCard
          title="PR Review Queue"
          value={quickStats?.pending_purchase_requests_count || 0}
          helper="Purchase requests waiting first-stage finance staff review"
          icon={FiClipboard}
          accentColor="orange"
        />
        <FinanceStaffStatCard
          title="Draft Journals"
          value={quickStats?.draft_journals_count || 0}
          helper="Journal entries still in draft before posting review"
          icon={FiFileText}
          accentColor="blue"
        />
        <FinanceStaffStatCard
          title="Open Vendor Bills"
          value={quickStats?.unpaid_purchase_count || 0}
          helper="Approved purchases with outstanding unpaid balance"
          icon={FiDollarSign}
          accentColor="red"
        />
        <FinanceStaffStatCard
          title="Transfers Waiting Manager"
          value={quickStats?.pending_transfer_requests_count || 0}
          helper="Transfer requests submitted by you and still pending approval"
          icon={FiRepeat}
          accentColor="purple"
        />
      </SimpleGrid>

      <Grid templateColumns={{ base: "1fr", xl: "1.2fr 1fr" }} gap={6} mb={6}>
        <GridItem>
          <Card
            bg={cardBg}
            border="1px"
            borderColor={cardBorderColor}
            boxShadow="sm"
            h="100%"
          >
            <CardHeader pb={2}>
              <Flex justify="space-between" align="start" gap={4}>
                <Box>
                  <Heading size="md" color={headingColor}>
                    Pending Purchase Request Review
                  </Heading>
                  <Text fontSize="sm" color={mutedTextColor}>
                    Prioritize requests that are waiting your first approval
                    step.
                  </Text>
                </Box>
                <Button
                  size="sm"
                  colorScheme="orange"
                  variant="outline"
                  onClick={() => router.push("/purchase-requests")}
                >
                  Open PR
                </Button>
              </Flex>
            </CardHeader>
            <CardBody pt={2}>
              <VStack spacing={3} align="stretch">
                {pendingPurchaseRequests.length > 0 ? (
                  pendingPurchaseRequests.map((request) => (
                    <Box
                      key={request.id}
                      p={4}
                      borderRadius="lg"
                      bg={softPanelBg}
                    >
                      <Flex justify="space-between" align="start" gap={3}>
                        <Box>
                          <HStack spacing={2} mb={1} wrap="wrap">
                            <Text fontWeight="semibold">
                              {request.pr_number}
                            </Text>
                            <Badge colorScheme="orange">Need Review</Badge>
                          </HStack>
                          <Text fontSize="sm" color={mutedTextColor}>
                            Requester: {request.requester_name}{" "}
                            {request.department
                              ? `- ${request.department}`
                              : ""}
                          </Text>
                          <Text fontSize="sm" color={mutedTextColor}>
                            Project: {request.project_name || "-"}
                          </Text>
                        </Box>
                        <Box textAlign="right">
                          <Text fontWeight="semibold">
                            {formatCurrency(request.total_estimated_amount)}
                          </Text>
                          <Text fontSize="xs" color={mutedTextColor}>
                            {new Date(request.created_at).toLocaleDateString(
                              "id-ID",
                            )}
                          </Text>
                        </Box>
                      </Flex>
                    </Box>
                  ))
                ) : (
                  <Text color={mutedTextColor}>
                    No purchase requests are waiting finance staff review right
                    now.
                  </Text>
                )}
              </VStack>
            </CardBody>
          </Card>
        </GridItem>

        <GridItem>
          <Card
            bg={cardBg}
            border="1px"
            borderColor={cardBorderColor}
            boxShadow="sm"
            h="100%"
          >
            <CardHeader pb={2}>
              <Heading size="md" color={headingColor}>
                Operational Snapshot
              </Heading>
              <Text fontSize="sm" color={mutedTextColor}>
                Team-level figures to support your daily processing priorities.
              </Text>
            </CardHeader>
            <CardBody pt={2}>
              <VStack spacing={4} align="stretch">
                <Box p={4} borderRadius="lg" bg={softPanelBg}>
                  <Text fontSize="sm" color={mutedTextColor}>
                    Outstanding Receivables
                  </Text>
                  <Heading size="md" mt={1}>
                    {formatCurrency(snapshot?.outstanding_receivables || 0)}
                  </Heading>
                </Box>
                <Box p={4} borderRadius="lg" bg={softPanelBg}>
                  <Text fontSize="sm" color={mutedTextColor}>
                    Outstanding Payables
                  </Text>
                  <Heading size="md" mt={1}>
                    {formatCurrency(snapshot?.outstanding_payables || 0)}
                  </Heading>
                </Box>
                <Box p={4} borderRadius="lg" bg={softPanelBg}>
                  <Text fontSize="sm" color={mutedTextColor}>
                    Cash & Bank Position
                  </Text>
                  <Heading size="md" mt={1}>
                    {formatCurrency(snapshot?.cash_bank_balance || 0)}
                  </Heading>
                </Box>
                <Box
                  p={4}
                  borderRadius="lg"
                  bg="green.50"
                  border="1px solid"
                  borderColor="green.200"
                >
                  <Text fontSize="sm" color="green.700">
                    Reviewed This Month
                  </Text>
                  <Heading size="md" mt={1} color="green.700">
                    {quickStats?.monthly_reviewed_purchase_count || 0} PR
                  </Heading>
                </Box>
                <SimpleGrid columns={{ base: 1, md: 2 }} spacing={3}>
                  <Button
                    leftIcon={<FiClipboard />}
                    colorScheme="orange"
                    variant="outline"
                    onClick={() => router.push("/purchase-requests")}
                  >
                    Review PR
                  </Button>
                  <Button
                    leftIcon={<FiDollarSign />}
                    colorScheme="red"
                    variant="outline"
                    onClick={() => router.push("/payments")}
                  >
                    Open Payments
                  </Button>
                  <Button
                    leftIcon={<FiFileText />}
                    colorScheme="blue"
                    variant="outline"
                    onClick={() => router.push("/journal")}
                  >
                    Draft Journal
                  </Button>
                  <Button
                    leftIcon={<FiRepeat />}
                    colorScheme="purple"
                    variant="outline"
                    onClick={() => router.push("/cash-bank")}
                  >
                    Cash & Bank
                  </Button>
                </SimpleGrid>
              </VStack>
            </CardBody>
          </Card>
        </GridItem>
      </Grid>

      <Grid templateColumns={{ base: "1fr", xl: "1fr 1fr" }} gap={6}>
        <GridItem>
          <Card
            bg={cardBg}
            border="1px"
            borderColor={cardBorderColor}
            boxShadow="sm"
            h="100%"
          >
            <CardHeader pb={2}>
              <Flex justify="space-between" align="start" gap={4}>
                <Box>
                  <Heading size="md" color={headingColor}>
                    My Transfer Requests
                  </Heading>
                  <Text fontSize="sm" color={mutedTextColor}>
                    Monitor transfers that still need finance manager approval.
                  </Text>
                </Box>
                <Button
                  size="sm"
                  colorScheme="purple"
                  variant="outline"
                  onClick={() => router.push("/cash-bank")}
                >
                  Open Cash & Bank
                </Button>
              </Flex>
            </CardHeader>
            <CardBody pt={2}>
              <VStack spacing={3} align="stretch">
                {recentTransferRequests.length > 0 ? (
                  recentTransferRequests.map((transfer) => (
                    <Box
                      key={transfer.id}
                      p={4}
                      borderRadius="lg"
                      bg={softPanelBg}
                    >
                      <Flex justify="space-between" align="start" gap={3}>
                        <Box>
                          <HStack spacing={2} mb={1} wrap="wrap">
                            <Text fontWeight="semibold">
                              {transfer.transfer_number}
                            </Text>
                            <Badge
                              colorScheme={getTransferStatusColor(
                                transfer.status,
                              )}
                            >
                              {getTransferStatusLabel(transfer.status)}
                            </Badge>
                          </HStack>
                          <Text fontSize="sm" color={mutedTextColor}>
                            {transfer.from_account_name || "-"} to{" "}
                            {transfer.to_account_name || "-"}
                          </Text>
                          <Text fontSize="sm" color={mutedTextColor}>
                            {formatTransferAmount(transfer)}
                          </Text>
                          {transfer.reference && (
                            <Text fontSize="xs" color={mutedTextColor}>
                              Ref: {transfer.reference}
                            </Text>
                          )}
                          {transfer.rejection_reason && (
                            <Text fontSize="xs" color="red.500">
                              {transfer.rejection_reason}
                            </Text>
                          )}
                        </Box>
                        <Text fontSize="xs" color={mutedTextColor}>
                          {new Date(transfer.date).toLocaleDateString("id-ID")}
                        </Text>
                      </Flex>
                    </Box>
                  ))
                ) : (
                  <Text color={mutedTextColor}>
                    You have no recent transfer requests yet.
                  </Text>
                )}
              </VStack>
            </CardBody>
          </Card>
        </GridItem>

        <GridItem>
          <Card
            bg={cardBg}
            border="1px"
            borderColor={cardBorderColor}
            boxShadow="sm"
            h="100%"
          >
            <CardHeader pb={2}>
              <Heading size="md" color={headingColor}>
                Recent Notifications
              </Heading>
              <Text fontSize="sm" color={mutedTextColor}>
                Keep an eye on approval, transfer, and document updates
                addressed to you.
              </Text>
            </CardHeader>
            <CardBody pt={2}>
              <VStack spacing={3} align="stretch">
                {recentNotifications.length > 0 ? (
                  recentNotifications.map((notification) => (
                    <Box
                      key={notification.id}
                      p={4}
                      borderRadius="lg"
                      bg={notification.is_read ? softPanelBg : "blue.50"}
                      border={notification.is_read ? "none" : "1px solid"}
                      borderColor={
                        notification.is_read ? "transparent" : "blue.200"
                      }
                    >
                      <Flex justify="space-between" align="start" gap={3}>
                        <Box>
                          <HStack spacing={2} mb={1} wrap="wrap">
                            <Text fontWeight="semibold">
                              {notification.title}
                            </Text>
                            <Badge
                              colorScheme={getNotificationColor(
                                notification.priority,
                              )}
                            >
                              {notification.priority || "NORMAL"}
                            </Badge>
                            {!notification.is_read && (
                              <Badge colorScheme="blue">Unread</Badge>
                            )}
                          </HStack>
                          <Text fontSize="sm" color={mutedTextColor}>
                            {notification.message}
                          </Text>
                        </Box>
                        <Icon
                          as={FiBell}
                          color={notification.is_read ? "gray.400" : "blue.500"}
                          mt={1}
                        />
                      </Flex>
                      <Text fontSize="xs" color={mutedTextColor} mt={2}>
                        {new Date(notification.created_at).toLocaleString(
                          "id-ID",
                        )}
                      </Text>
                    </Box>
                  ))
                ) : (
                  <Text color={mutedTextColor}>
                    No recent notifications for this account.
                  </Text>
                )}

                <Box
                  p={4}
                  borderRadius="lg"
                  bg="yellow.50"
                  border="1px solid"
                  borderColor="yellow.200"
                >
                  <HStack spacing={2} mb={1}>
                    <Icon as={FiClock} color="yellow.600" />
                    <Text fontWeight="semibold" color="yellow.700">
                      Unread Notifications
                    </Text>
                  </HStack>
                  <Text color="yellow.700">
                    {quickStats?.unread_notifications_count || 0} notifications
                    still need your attention.
                  </Text>
                </Box>
              </VStack>
            </CardBody>
          </Card>
        </GridItem>
      </Grid>
    </Box>
  );
};

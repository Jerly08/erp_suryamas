"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
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
  Heading,
  HStack,
  Icon,
  SimpleGrid,
  Spinner,
  Text,
  VStack,
  useColorModeValue,
} from "@chakra-ui/react";
import { FiCheckSquare, FiExternalLink, FiFileText } from "react-icons/fi";
import purchaseService, {
  Purchase,
  PurchaseSummary,
} from "@/services/purchaseService";

interface ApiErrorShape {
  response?: {
    data?: {
      error?: string;
    };
  };
  message?: string;
}

const formatCurrency = (amount: number): string =>
  new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(amount || 0);

const formatDate = (value?: string): string => {
  if (!value) return "-";

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "-";

  return parsed.toLocaleDateString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
  });
};

const getPurchaseRequestNumber = (purchase: Purchase): string =>
  purchase.purchase_request?.pr_number ||
  purchase.purchase_request?.request_number ||
  (purchase.purchase_request_id ? `#${purchase.purchase_request_id}` : "");

export const COAApprovalQueueCard: React.FC = () => {
  const router = useRouter();
  const [purchases, setPurchases] = useState<Purchase[]>([]);
  const [summary, setSummary] = useState<PurchaseSummary | null>(null);
  const [totalQueue, setTotalQueue] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const cardBg = useColorModeValue("white", "gray.800");
  const cardBorderColor = useColorModeValue("gray.200", "whiteAlpha.200");
  const panelBg = useColorModeValue("orange.50", "orange.900");
  const softPanelBg = useColorModeValue("gray.50", "whiteAlpha.50");
  const headingColor = useColorModeValue("gray.800", "whiteAlpha.900");
  const mutedTextColor = useColorModeValue("gray.500", "whiteAlpha.700");

  const openCOAQueue = () => {
    router.push("/purchases?accounting_status=READY_FOR_COA");
  };

  useEffect(() => {
    const fetchCOAQueue = async () => {
      try {
        setLoading(true);
        const [listResponse, summaryResponse] = await Promise.all([
          purchaseService.list({
            accounting_status: "READY_FOR_COA",
            page: 1,
            limit: 5,
          }),
          purchaseService.getSummary({
            accounting_status: "READY_FOR_COA",
          }),
        ]);

        setPurchases(Array.isArray(listResponse.data) ? listResponse.data : []);
        setTotalQueue(listResponse.total || 0);
        setSummary(summaryResponse);
        setError(null);
      } catch (fetchError: unknown) {
        console.error("Error fetching COA approval queue:", fetchError);
        const normalizedError = fetchError as ApiErrorShape;
        setError(
          normalizedError.response?.data?.error ||
            normalizedError.message ||
            "Gagal memuat antrean COA approval",
        );
      } finally {
        setLoading(false);
      }
    };

    void fetchCOAQueue();
  }, []);

  return (
    <Card
      bg={cardBg}
      border="1px"
      borderColor={cardBorderColor}
      boxShadow="sm"
      w="full"
    >
      <CardHeader pb={2}>
        <Flex
          justify="space-between"
          align={{ base: "flex-start", md: "center" }}
          gap={4}
          direction={{ base: "column", md: "row" }}
        >
          <HStack spacing={3} align="start">
            <Flex
              align="center"
              justify="center"
              w={11}
              h={11}
              borderRadius="xl"
              bg={panelBg}
              color="orange.500"
              flexShrink={0}
            >
              <Icon as={FiCheckSquare} boxSize={5} />
            </Flex>
            <Box>
              <Heading size="md" color={headingColor}>
                COA Approval Queue
              </Heading>
              <Text fontSize="sm" color={mutedTextColor}>
                Purchase yang sudah approved Finance Manager dan menunggu input
                GL/COA oleh Accounting.
              </Text>
            </Box>
          </HStack>
          <Button
            rightIcon={<FiExternalLink />}
            colorScheme="orange"
            variant="outline"
            size="sm"
            onClick={openCOAQueue}
          >
            Buka Purchases
          </Button>
        </Flex>
      </CardHeader>

      <CardBody pt={2}>
        {loading ? (
          <Flex align="center" justify="center" py={8}>
            <Spinner color="orange.500" mr={3} />
            <Text color={mutedTextColor}>Memuat antrean COA...</Text>
          </Flex>
        ) : error ? (
          <Alert status="warning" borderRadius="md">
            <AlertIcon />
            {error}
          </Alert>
        ) : (
          <VStack spacing={4} align="stretch">
            <SimpleGrid columns={{ base: 1, md: 3 }} spacing={3}>
              <Box p={4} borderRadius="lg" bg={panelBg}>
                <Text fontSize="sm" color={mutedTextColor}>
                  Butuh COA Approval
                </Text>
                <Heading size="lg" color={headingColor}>
                  {totalQueue}
                </Heading>
              </Box>
              <Box p={4} borderRadius="lg" bg={softPanelBg}>
                <Text fontSize="sm" color={mutedTextColor}>
                  Total Nominal
                </Text>
                <Heading size="md" color={headingColor}>
                  {formatCurrency(summary?.total_amount || 0)}
                </Heading>
              </Box>
              <Box p={4} borderRadius="lg" bg={softPanelBg}>
                <Text fontSize="sm" color={mutedTextColor}>
                  Status Monitoring
                </Text>
                <Badge colorScheme={totalQueue > 0 ? "orange" : "green"} mt={2}>
                  {totalQueue > 0 ? "Perlu Ditindaklanjuti" : "Bersih"}
                </Badge>
              </Box>
            </SimpleGrid>

            {purchases.length > 0 ? (
              <VStack spacing={3} align="stretch">
                {purchases.map((purchase) => (
                  <Box
                    key={purchase.id}
                    p={4}
                    borderRadius="lg"
                    bg={softPanelBg}
                    borderWidth="1px"
                    borderColor={cardBorderColor}
                  >
                    <Flex
                      justify="space-between"
                      gap={4}
                      align={{ base: "flex-start", md: "center" }}
                      direction={{ base: "column", md: "row" }}
                    >
                      <HStack align="start" spacing={3}>
                        <Icon as={FiFileText} color="orange.500" mt={1} />
                        <Box>
                          <HStack spacing={2} wrap="wrap">
                            <Text fontWeight="semibold">{purchase.code}</Text>
                            <Badge colorScheme="orange">READY FOR COA</Badge>
                          </HStack>
                          <Text fontSize="sm" color={mutedTextColor}>
                            Vendor: {purchase.vendor?.name || "-"} | Tanggal:{" "}
                            {formatDate(purchase.date)}
                          </Text>
                          {purchase.purchase_request_id && (
                            <VStack align="start" spacing={0.5} mt={2}>
                              <Text fontSize="xs" color={mutedTextColor}>
                                PR: {getPurchaseRequestNumber(purchase)}
                              </Text>
                              {purchase.purchase_request?.title && (
                                <Text
                                  fontSize="xs"
                                  color={headingColor}
                                  fontWeight="semibold"
                                  noOfLines={2}
                                >
                                  Judul PR: {purchase.purchase_request.title}
                                </Text>
                              )}
                              {purchase.purchase_request?.department && (
                                <Text fontSize="xs" color={mutedTextColor}>
                                  Divisi: {purchase.purchase_request.department}
                                </Text>
                              )}
                            </VStack>
                          )}
                        </Box>
                      </HStack>
                      <Box textAlign={{ base: "left", md: "right" }}>
                        <Text fontWeight="bold">
                          {formatCurrency(purchase.total_amount)}
                        </Text>
                        <Text fontSize="xs" color={mutedTextColor}>
                          {purchase.purchase_items?.length || 0} item
                        </Text>
                      </Box>
                    </Flex>
                  </Box>
                ))}
              </VStack>
            ) : (
              <Box p={4} borderRadius="lg" bg={softPanelBg}>
                <Text color={mutedTextColor}>
                  Tidak ada purchase yang menunggu input COA saat ini.
                </Text>
              </Box>
            )}
          </VStack>
        )}
      </CardBody>
    </Card>
  );
};

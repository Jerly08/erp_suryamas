"use client";

import React, { useState, useEffect, useRef, useCallback } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { useAuth } from "@/contexts/AuthContext";
import { useTranslation } from "@/hooks/useTranslation";
import UnifiedLayout from "@/components/layout/UnifiedLayout";
import salesService, { Sale } from "@/services/salesService";
import PaymentForm from "@/components/sales/PaymentForm";
import PPh23SettlementForm from "@/components/sales/PPh23SettlementForm";
import ConvertConfirmedSaleToCreditModal from "@/components/sales/ConvertConfirmedSaleToCreditModal";
import { PostedTransactionCorrectionsPanel } from "@/components/corrections/PostedTransactionCorrections";
import { getSalePPhStatus } from "@/utils/taxPaymentStatus";
import {
  isAccountingRole,
  isFinanceManagerRole,
  isFinanceStaffRole,
  normalizeRole,
} from "@/utils/roles";
import {
  Box,
  Heading,
  Text,
  Button,
  Flex,
  HStack,
  VStack,
  Card,
  CardHeader,
  CardBody,
  Badge,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  TableContainer,
  Divider,
  Grid,
  GridItem,
  Spinner,
  Alert,
  AlertIcon,
  useToast,
  useDisclosure,
  IconButton,
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  Tooltip,
} from "@chakra-ui/react";
import {
  FiArrowLeft,
  FiEdit,
  FiDollarSign,
  FiDownload,
  FiMoreVertical,
  FiCheck,
  FiX,
  FiFileText,
} from "react-icons/fi";

const SaleDetailPage: React.FC = () => {
  const { t } = useTranslation();
  const { user } = useAuth();
  const params = useParams();
  const router = useRouter();
  const searchParams = useSearchParams();
  const toast = useToast();
  const {
    isOpen: isPaymentOpen,
    onOpen: onPaymentOpen,
    onClose: onPaymentClose,
  } = useDisclosure();
  const {
    isOpen: isPPh23Open,
    onOpen: onPPh23Open,
    onClose: onPPh23Close,
  } = useDisclosure();
  const {
    isOpen: isConvertCreditOpen,
    onOpen: onConvertCreditOpen,
    onClose: onConvertCreditClose,
  } = useDisclosure();

  const [sale, setSale] = useState<Sale | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState(false);
  const correctionSectionRef = useRef<HTMLDivElement>(null);
  const hasAutoOpenedCreditConversionRef = useRef(false);

  const saleId = params?.id as string;
  const correctionFocus = searchParams?.get("focus");
  const correctionRole = normalizeRole(user?.role);
  const canUseCreditCorrection =
    correctionRole === "admin" ||
    isFinanceStaffRole(correctionRole) ||
    isAccountingRole(correctionRole) ||
    isFinanceManagerRole(correctionRole);
  const canConvertConfirmedSaleToCredit =
    correctionRole === "admin" || isFinanceManagerRole(correctionRole);
  const canConvertThisSaleToCredit =
    canConvertConfirmedSaleToCredit &&
    sale?.status === "CONFIRMED" &&
    !sale?.invoice_number;

  // Handle back navigation - simplified and more reliable
  const handleGoBack = () => {
    console.log("Back button clicked");
    // Direct navigation to sales page - most reliable
    router.push("/sales");
  };

  // Load sale data
  const loadSale = async () => {
    if (!saleId) return;

    try {
      setLoading(true);
      setError(null);
      const saleData = await salesService.getSale(parseInt(saleId));
      setSale(saleData);
    } catch (error: any) {
      setError(error.response?.data?.message || t("sales.detail.loadingError"));
      toast({
        title: t("common.error"),
        description:
          error.response?.data?.message || t("sales.detail.loadingError"),
        status: "error",
        duration: 3000,
      });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadSale();
  }, [saleId]);

  const focusCreditCorrection = useCallback(() => {
    correctionSectionRef.current?.scrollIntoView({
      behavior: "smooth",
      block: "start",
    });
    correctionSectionRef.current?.focus({ preventScroll: true });
  }, []);

  useEffect(() => {
    if (
      correctionFocus !== "convert-credit" ||
      hasAutoOpenedCreditConversionRef.current ||
      !canConvertThisSaleToCredit
    ) {
      return;
    }

    hasAutoOpenedCreditConversionRef.current = true;
    const timeoutId = window.setTimeout(onConvertCreditOpen, 100);
    return () => window.clearTimeout(timeoutId);
  }, [
    canConvertThisSaleToCredit,
    correctionFocus,
    onConvertCreditOpen,
  ]);

  useEffect(() => {
    if (
      loading ||
      !sale?.invoice_number ||
      correctionFocus !== "credit-correction"
    ) {
      return;
    }

    const timeoutId = window.setTimeout(focusCreditCorrection, 100);
    return () => window.clearTimeout(timeoutId);
  }, [
    correctionFocus,
    focusCreditCorrection,
    loading,
    sale?.id,
    sale?.invoice_number,
  ]);

  // Handle status actions
  const handleStatusAction = async (action: "confirm" | "cancel") => {
    if (!sale) return;

    try {
      setActionLoading(true);

      if (action === "confirm") {
        await salesService.confirmSale(sale.id);
        toast({
          title: t("sales.detail.confirmSuccess"),
          description: t("sales.detail.confirmSuccess"),
          status: "success",
          duration: 3000,
        });
      } else if (action === "cancel") {
        const reason = window.prompt(t("sales.detail.cancelReason"));
        if (reason) {
          await salesService.cancelSale(sale.id, reason);
          toast({
            title: t("sales.detail.cancelSuccess"),
            description: t("sales.detail.cancelSuccess"),
            status: "success",
            duration: 3000,
          });
        } else {
          return;
        }
      }

      loadSale(); // Reload to get updated status
    } catch (error: any) {
      toast({
        title: t("common.error"),
        description:
          error.response?.data?.message || t(`sales.detail.${action}Error`),
        status: "error",
        duration: 3000,
      });
    } finally {
      setActionLoading(false);
    }
  };

  // Handle create invoice
  const handleCreateInvoice = async () => {
    if (!sale) return;

    try {
      setActionLoading(true);
      await salesService.createInvoiceFromSale(sale.id);
      toast({
        title: t("sales.detail.invoiceSuccess"),
        description: t("sales.detail.invoiceSuccess"),
        status: "success",
        duration: 3000,
      });
      loadSale(); // Reload to get updated data
    } catch (error: any) {
      toast({
        title: t("common.error"),
        description:
          error.response?.data?.message || t("sales.detail.invoiceError"),
        status: "error",
        duration: 3000,
      });
    } finally {
      setActionLoading(false);
    }
  };

  // Handle payment form save
  const handlePaymentSave = () => {
    onPaymentClose();
    loadSale(); // Reload to get updated payment status
  };

  const handlePPh23Save = () => {
    onPPh23Close();
    loadSale();
  };

  const displayStatus = sale?.settlement_status || sale?.status || "DRAFT";
  const canAddPPh23 =
    !!sale &&
    (sale.pph23_amount || 0) <= 0 &&
    !sale.pph23_settlement_date &&
    sale.status !== "DRAFT" &&
    sale.status !== "PENDING" &&
    sale.status !== "CONFIRMED" &&
    sale.status !== "CANCELLED";
  const canPostPPh23 =
    !!sale &&
    (sale.pph23_amount || 0) > 0 &&
    !sale.pph23_settlement_date &&
    sale.status !== "CANCELLED";
  const pph23PostingStatus = !sale
    ? "-"
    : sale.pph23_settlement_date
      ? `Sudah dipost ${salesService.formatDate(sale.pph23_settlement_date)}`
      : canAddPPh23
        ? "Belum ditambahkan"
        : (sale.pph23_amount || 0) <= 0
          ? "Tidak ada PPh23"
          : "Siap dipost";
  const pphStatus = sale ? getSalePPhStatus(sale) : null;
  const hasOutstandingAmount = !!sale && (sale.outstanding_amount || 0) > 0;
  const canReceivePayment =
    hasOutstandingAmount &&
    (sale?.status === "INVOICED" || sale?.status === "OVERDUE");
  const paymentMethodType = String(sale?.payment_method_type || "").toUpperCase();
  const isImmediatePaymentMethod =
    paymentMethodType === "CASH" || paymentMethodType === "BANK";
  const immediatePaymentMethodLabel =
    paymentMethodType === "CASH"
      ? t("sales.detail.paymentMethodCash")
      : t("sales.detail.paymentMethodBank");
  const isConfirmedImmediatePayment =
    !!sale && sale.status === "CONFIRMED" && isImmediatePaymentMethod;
  const showDeferredPaymentHint =
    hasOutstandingAmount && !canReceivePayment && !isConfirmedImmediatePayment;
  const saleItems = sale?.sale_items || [];
  const pendingBackorderQty = saleItems.reduce(
    (sum, item) => sum + (item.backorder_quantity || 0),
    0,
  );
  const fulfilledBackorderQty = saleItems.reduce(
    (sum, item) =>
      sum +
      Math.max(
        (item.fulfilled_quantity || 0) - (item.available_quantity || 0),
        0,
      ),
    0,
  );

  // Get status color
  const getStatusColor = (status: string) => {
    switch (status?.toLowerCase()) {
      case "paid":
        return "green";
      case "partially_paid":
        return "orange";
      case "invoiced":
        return "blue";
      case "confirmed":
        return "purple";
      case "overdue":
        return "red";
      case "draft":
        return "gray";
      case "cancelled":
        return "red";
      default:
        return "gray";
    }
  };

  // Show loading state
  if (loading) {
    return (
      <UnifiedLayout>
        <Flex justify="center" align="center" minH="400px">
          <Spinner size="xl" />
        </Flex>
      </UnifiedLayout>
    );
  }

  // Show error state
  if (error || !sale) {
    return (
      <UnifiedLayout>
        <Alert status="error">
          <AlertIcon />
          {error || t("sales.detail.saleNotFound")}
        </Alert>
      </UnifiedLayout>
    );
  }

  return (
    <UnifiedLayout>
      <VStack spacing={6} align="stretch">
        {/* Header */}
        <Flex justify="space-between" align="center">
          <HStack spacing={4}>
            <IconButton
              icon={<FiArrowLeft />}
              variant="outline"
              onClick={handleGoBack}
              aria-label={t("sales.detail.goBack")}
              _hover={{
                bg: "var(--bg-tertiary)",
                transform: "translateX(-2px)",
                borderColor: "var(--accent-color)",
              }}
              size="md"
              title={t("sales.detail.goBack")}
              borderColor="var(--border-color)"
              color="var(--text-primary)"
              bg="var(--bg-secondary)"
              cursor="pointer"
            />
            <VStack align="start" spacing={1}>
              <Heading as="h1" size="xl">
                {t("sales.detail.title")}
              </Heading>
              <HStack spacing={2}>
                <Text color="gray.600">
                  {t("sales.detail.code")}: {sale.code}
                </Text>
                <Badge
                  colorScheme={getStatusColor(displayStatus)}
                  variant="subtle"
                >
                  {salesService.getStatusLabel(displayStatus)}
                </Badge>
              </HStack>
            </VStack>
          </HStack>

          <HStack spacing={3}>
            <Button
              leftIcon={<FiDownload />}
              variant="outline"
              onClick={() =>
                salesService.downloadInvoicePDF(
                  sale.id,
                  sale.invoice_number || sale.code,
                )
              }
            >
              {t("sales.detail.downloadPDF")}
            </Button>

            <Menu>
              <MenuButton
                as={IconButton}
                icon={<FiMoreVertical />}
                variant="outline"
              />
              <MenuList>
                {sale.status === "DRAFT" && (
                  <MenuItem
                    icon={<FiCheck />}
                    onClick={() => handleStatusAction("confirm")}
                    isDisabled={actionLoading}
                  >
                    {t("sales.detail.confirmSale")}
                  </MenuItem>
                )}
                {sale.status === "CONFIRMED" && (
                  <MenuItem
                    icon={<FiFileText />}
                    onClick={handleCreateInvoice}
                    isDisabled={actionLoading}
                  >
                    {isConfirmedImmediatePayment
                      ? t("sales.detail.createInvoiceAndReceivePayment")
                      : t("sales.detail.createInvoice")}
                  </MenuItem>
                )}
                {canReceivePayment && (
                  <MenuItem icon={<FiDollarSign />} onClick={onPaymentOpen}>
                    {t("sales.detail.recordPayment")}
                  </MenuItem>
                )}
                {showDeferredPaymentHint && (
                  <Tooltip
                    label={t("sales.detail.paymentRequiresInvoice")}
                    placement="left"
                    hasArrow
                    shouldWrapChildren
                  >
                    <MenuItem icon={<FiDollarSign />} isDisabled>
                      {t("sales.detail.recordPayment")}
                    </MenuItem>
                  </Tooltip>
                )}
                {(canAddPPh23 || canPostPPh23) && (
                  <MenuItem icon={<FiCheck />} onClick={onPPh23Open}>
                    {canAddPPh23
                      ? "Tambah PPh23"
                      : "Posting PPh23"}
                  </MenuItem>
                )}
                {sale.invoice_number && canUseCreditCorrection && (
                  <MenuItem icon={<FiEdit />} onClick={focusCreditCorrection}>
                    Koreksi Piutang Kredit Lama
                  </MenuItem>
                )}
                {canConvertThisSaleToCredit && (
                  <MenuItem icon={<FiEdit />} onClick={onConvertCreditOpen}>
                    Ubah Menjadi Penjualan Kredit
                  </MenuItem>
                )}
                <MenuItem
                  icon={<FiEdit />}
                  onClick={() => router.push(`/sales?edit=${sale.id}`)}
                >
                  {t("sales.detail.editSale")}
                </MenuItem>
                <Divider />
                {sale.status !== "CANCELLED" && (
                  <MenuItem
                    icon={<FiX />}
                    color="red.500"
                    onClick={() => handleStatusAction("cancel")}
                    isDisabled={actionLoading}
                  >
                    {t("sales.detail.cancelSale")}
                  </MenuItem>
                )}
              </MenuList>
            </Menu>
          </HStack>
        </Flex>

        {isConfirmedImmediatePayment && (
          <Alert status="info" borderRadius="md" alignItems="flex-start">
            <AlertIcon mt={1} />
            <Box>
              <Text fontWeight="semibold">
                {t("sales.detail.immediatePaymentPendingTitle")}
              </Text>
              <Text fontSize="sm">
                {t("sales.detail.immediatePaymentPendingDesc", {
                  method: immediatePaymentMethodLabel,
                })}
              </Text>
            </Box>
          </Alert>
        )}

        {/* Basic Information */}
        <Card>
          <CardHeader>
            <Heading size="md">{t("sales.detail.saleInformation")}</Heading>
          </CardHeader>
          <CardBody>
            <Grid templateColumns="repeat(3, 1fr)" gap={6}>
              <GridItem>
                <VStack align="start" spacing={2}>
                  <Text fontSize="sm" color="gray.600">
                    {t("sales.detail.customer")}
                  </Text>
                  <Text fontWeight="medium">
                    {sale.customer
                      ? sale.customer.name
                      : t("sales.detail.notAvailable")}
                  </Text>
                </VStack>
              </GridItem>
              <GridItem>
                <VStack align="start" spacing={2}>
                  <Text fontSize="sm" color="gray.600">
                    {t("sales.detail.invoiceNumber")}
                  </Text>
                  <Text fontWeight="medium">
                    {sale.invoice_number
                      ? sale.invoice_number
                      : t("sales.detail.noInvoiceNumber")}
                  </Text>
                </VStack>
              </GridItem>
              <GridItem>
                <VStack align="start" spacing={2}>
                  <Text fontSize="sm" color="gray.600">
                    {t("sales.detail.salesPerson")}
                  </Text>
                  <Text fontWeight="medium">
                    {sale.sales_person
                      ? sale.sales_person.name
                      : t("sales.detail.notAvailable")}
                  </Text>
                </VStack>
              </GridItem>
              <GridItem>
                <VStack align="start" spacing={2}>
                  <Text fontSize="sm" color="gray.600">
                    {t("sales.detail.date")}
                  </Text>
                  <Text fontWeight="medium">
                    {salesService.formatDate(sale.date)}
                  </Text>
                </VStack>
              </GridItem>
              <GridItem>
                <VStack align="start" spacing={2}>
                  <Text fontSize="sm" color="gray.600">
                    {t("sales.detail.dueDate")}
                  </Text>
                  <Text fontWeight="medium">
                    {sale.due_date && sale.due_date !== "0001-01-01T00:00:00Z"
                      ? salesService.formatDate(sale.due_date)
                      : t("sales.detail.noDueDate")}
                  </Text>
                </VStack>
              </GridItem>
              <GridItem>
                <VStack align="start" spacing={2}>
                  <Text fontSize="sm" color="gray.600">
                    {t("sales.detail.paymentTerms")}
                  </Text>
                  <Text fontWeight="medium">
                    {sale.payment_terms
                      ? sale.payment_terms
                      : t("sales.detail.notAvailable")}
                  </Text>
                </VStack>
              </GridItem>
            </Grid>
          </CardBody>
        </Card>

        {/* Items */}
        <Card>
          <CardHeader>
            <Heading size="md">{t("sales.detail.saleItems")}</Heading>
          </CardHeader>
          <CardBody>
            {(pendingBackorderQty > 0 || fulfilledBackorderQty > 0) && (
              <Alert
                status={pendingBackorderQty > 0 ? "warning" : "success"}
                mb={4}
                borderRadius="md"
              >
                <AlertIcon />
                {pendingBackorderQty > 0
                  ? `${pendingBackorderQty} qty masih backorder. ${fulfilledBackorderQty} qty backorder sudah dipenuhi.`
                  : `${fulfilledBackorderQty} qty backorder sudah dipenuhi oleh purchase berikutnya.`}
              </Alert>
            )}
            <TableContainer>
              <Table variant="simple">
                <Thead>
                  <Tr>
                    <Th>{t("sales.detail.product")}</Th>
                    <Th>{t("sales.detail.description")}</Th>
                    <Th isNumeric>{t("sales.detail.quantity")}</Th>
                    <Th>Fulfillment</Th>
                    <Th isNumeric>{t("sales.detail.unitPrice")}</Th>
                    <Th isNumeric>{t("sales.detail.discount")}</Th>
                    <Th isNumeric>{t("sales.detail.total")}</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {sale.sale_items?.map((item, index) => (
                    <Tr key={index}>
                      <Td fontWeight="medium">
                        {item.product_name ||
                          item.product?.name ||
                          t("sales.detail.notAvailable")}
                      </Td>
                      <Td>
                        {item.description || t("sales.detail.noInvoiceNumber")}
                      </Td>
                      <Td isNumeric>{item.quantity || 0}</Td>
                      <Td>
                        <VStack align="start" spacing={1}>
                          <Badge
                            colorScheme={
                              item.backorder_quantity &&
                              item.backorder_quantity > 0
                                ? "orange"
                                : item.fulfillment_status === "FULFILLED"
                                  ? "green"
                                  : "green"
                            }
                            variant="subtle"
                          >
                            {item.backorder_quantity &&
                            item.backorder_quantity > 0
                              ? `${item.backorder_quantity} backorder`
                              : item.fulfillment_status === "FULFILLED"
                                ? "Fulfilled"
                                : "Ready stock"}
                          </Badge>
                          <Text fontSize="xs" color="gray.500">
                            {`Available ${item.available_quantity || 0} | Fulfilled ${item.fulfilled_quantity || 0}`}
                          </Text>
                        </VStack>
                      </Td>
                      <Td isNumeric>
                        {salesService.formatCurrency(item.unit_price || 0)}
                      </Td>
                      <Td isNumeric>
                        {item.discount_percent
                          ? `${item.discount_percent}%`
                          : t("sales.detail.noDiscount")}
                      </Td>
                      <Td isNumeric fontWeight="medium">
                        {salesService.formatCurrency(
                          item.line_total || item.total_price,
                        )}
                      </Td>
                    </Tr>
                  ))}
                </Tbody>
              </Table>
            </TableContainer>
          </CardBody>
        </Card>

        {/* Financial Summary */}
        <Card>
          <CardHeader>
            <Heading size="md">{t("sales.detail.financialSummary")}</Heading>
          </CardHeader>
          <CardBody>
            <VStack spacing={4} align="stretch">
              {/* Subtotal */}
              <Flex justify="space-between">
                <Text>{t("sales.detail.subtotal")}:</Text>
                <Text fontWeight="medium">
                  {salesService.formatCurrency(sale.subtotal || 0)}
                </Text>
              </Flex>

              {/* Global Discount - only show if > 0 */}
              {(sale.discount_percent > 0 || sale.discount_amount > 0) && (
                <Flex justify="space-between">
                  <Text color="red.500">
                    {t("sales.detail.discountLabel")} (
                    {sale.discount_percent || 0}%):
                  </Text>
                  <Text fontWeight="medium" color="red.500">
                    - {salesService.formatCurrency(sale.discount_amount || 0)}
                  </Text>
                </Flex>
              )}

              {/* PPN - only show if > 0 */}
              {((sale.ppn_rate || sale.ppn_percent || 0) > 0 ||
                (sale.ppn_amount || sale.ppn || 0) > 0) && (
                <Flex justify="space-between">
                  <Text color="blue.500">
                    {t("sales.detail.ppn")} (
                    {sale.ppn_rate || sale.ppn_percent || 0}%):
                  </Text>
                  <Text fontWeight="medium" color="blue.500">
                    +{" "}
                    {salesService.formatCurrency(
                      sale.ppn_amount || sale.ppn || 0,
                    )}
                  </Text>
                </Flex>
              )}

              {/* Other Tax Additions - only show if > 0 */}
              {(sale.other_tax_additions || 0) > 0 && (
                <Flex justify="space-between">
                  <Text color="blue.500">
                    {t("sales.detail.otherTaxAdditions")}:
                  </Text>
                  <Text fontWeight="medium" color="blue.500">
                    +{" "}
                    {salesService.formatCurrency(sale.other_tax_additions || 0)}
                  </Text>
                </Flex>
              )}

              {/* PPh21 - only show if > 0 */}
              {((sale.pph21_rate || 0) > 0 || (sale.pph21_amount || 0) > 0) && (
                <Flex justify="space-between">
                  <Text color="orange.500">
                    {t("sales.detail.pph21")} ({sale.pph21_rate || 0}%):
                  </Text>
                  <Text fontWeight="medium" color="orange.500">
                    - {salesService.formatCurrency(sale.pph21_amount || 0)}
                  </Text>
                </Flex>
              )}

              {/* PPh23 - only show if > 0 */}
              {((sale.pph23_rate || 0) > 0 || (sale.pph23_amount || 0) > 0) && (
                <Flex justify="space-between">
                  <Text color="orange.500">
                    {t("sales.detail.pph23")} ({sale.pph23_rate || 0}%):
                  </Text>
                  <Text fontWeight="medium" color="orange.500">
                    - {salesService.formatCurrency(sale.pph23_amount || 0)}
                  </Text>
                </Flex>
              )}

              {pphStatus && (
                <Flex justify="space-between" align="start" gap={4}>
                  <Text color="orange.500">Status PPh:</Text>
                  <VStack align="end" spacing={1}>
                    <Badge colorScheme={pphStatus.colorScheme} variant="subtle">
                      {pphStatus.label}
                    </Badge>
                    {pphStatus.totalAmount > 0 && (
                      <Text fontSize="xs" color="orange.500" textAlign="end">
                        Total {salesService.formatCurrency(pphStatus.totalAmount)}
                        {pphStatus.outstandingAmount > 0
                          ? `, sisa ${salesService.formatCurrency(
                              pphStatus.outstandingAmount,
                            )}`
                          : ""}
                      </Text>
                    )}
                  </VStack>
                </Flex>
              )}

              {(canAddPPh23 || (sale.pph23_amount || 0) > 0) && (
                <Flex justify="space-between">
                  <Text color="orange.500">Status Posting PPh23:</Text>
                  <Text fontWeight="medium" color="orange.500">
                    {pph23PostingStatus}
                  </Text>
                </Flex>
              )}

              {!!sale.pph23_settlement_reference && (
                <Flex justify="space-between" gap={4}>
                  <Text color="orange.500">Referensi PPh23:</Text>
                  <Text fontWeight="medium" color="orange.500" textAlign="end">
                    {sale.pph23_settlement_reference}
                  </Text>
                </Flex>
              )}

              {!!sale.pph23_settlement_notes && (
                <Flex justify="space-between" gap={4}>
                  <Text color="orange.500">Catatan Posting PPh23:</Text>
                  <Text fontWeight="medium" color="orange.500" textAlign="end">
                    {sale.pph23_settlement_notes}
                  </Text>
                </Flex>
              )}

              {/* Other Tax Deductions - only show if > 0 */}
              {(sale.other_tax_deductions || 0) > 0 && (
                <Flex justify="space-between">
                  <Text color="orange.500">
                    {t("sales.detail.otherTaxDeductions")}:
                  </Text>
                  <Text fontWeight="medium" color="orange.500">
                    -{" "}
                    {salesService.formatCurrency(
                      sale.other_tax_deductions || 0,
                    )}
                  </Text>
                </Flex>
              )}

              {/* Shipping Cost - only show if > 0 */}
              {(sale.shipping_cost || 0) > 0 && (
                <Flex justify="space-between">
                  <Text>{t("sales.detail.shippingCost")}:</Text>
                  <Text fontWeight="medium">
                    + {salesService.formatCurrency(sale.shipping_cost || 0)}
                  </Text>
                </Flex>
              )}

              <Divider />
              <Flex justify="space-between" fontSize="lg">
                <Text fontWeight="bold">{t("sales.detail.totalAmount")}:</Text>
                <Text fontWeight="bold" color="blue.600">
                  {salesService.formatCurrency(sale.total_amount || 0)}
                </Text>
              </Flex>
              <Flex justify="space-between">
                <Text color="green.600">{t("sales.detail.paidAmount")}:</Text>
                <Text fontWeight="medium" color="green.600">
                  {salesService.formatCurrency(sale.paid_amount || 0)}
                </Text>
              </Flex>
              <Flex justify="space-between" fontSize="lg">
                <Text fontWeight="bold" color="orange.600">
                  {t("sales.detail.outstanding")}:
                </Text>
                <Text fontWeight="bold" color="orange.600">
                  {salesService.formatCurrency(sale.outstanding_amount || 0)}
                </Text>
              </Flex>
            </VStack>
          </CardBody>
        </Card>

        {sale.invoice_number && canUseCreditCorrection && (
          <Card
            ref={correctionSectionRef}
            id="credit-correction"
            tabIndex={-1}
            scrollMarginTop="24px"
          >
            <CardHeader>
              <Heading size="md">Koreksi Transaksi Terposting</Heading>
            </CardHeader>
            <CardBody>
              <PostedTransactionCorrectionsPanel
                target={{
                  saleId: sale.id,
                  code: sale.invoice_number || sale.code,
                }}
                onChanged={() => {
                  void loadSale();
                }}
              />
            </CardBody>
          </Card>
        )}

        <ConvertConfirmedSaleToCreditModal
          isOpen={isConvertCreditOpen}
          onClose={onConvertCreditClose}
          sale={sale}
          onSuccess={() => {
            void loadSale();
          }}
        />

        {/* Notes */}
        {(sale.notes || sale.internal_notes) && (
          <Card>
            <CardHeader>
              <Heading size="md">{t("sales.detail.notes")}</Heading>
            </CardHeader>
            <CardBody>
              <VStack spacing={4} align="stretch">
                {sale.notes && (
                  <Box>
                    <Text fontSize="sm" color="gray.600" mb={2}>
                      {t("sales.detail.customerNotes")}:
                    </Text>
                    <Text>{sale.notes}</Text>
                  </Box>
                )}
                {sale.internal_notes && (
                  <Box>
                    <Text fontSize="sm" color="gray.600" mb={2}>
                      {t("sales.detail.internalNotes")}:
                    </Text>
                    <Text>{sale.internal_notes}</Text>
                  </Box>
                )}
              </VStack>
            </CardBody>
          </Card>
        )}
      </VStack>

      {/* Payment Form Modal */}
      <PaymentForm
        isOpen={isPaymentOpen}
        onClose={onPaymentClose}
        onSave={handlePaymentSave}
        sale={sale}
      />
      <PPh23SettlementForm
        isOpen={isPPh23Open}
        onClose={onPPh23Close}
        onSave={handlePPh23Save}
        sale={sale}
      />
    </UnifiedLayout>
  );
};

export default SaleDetailPage;

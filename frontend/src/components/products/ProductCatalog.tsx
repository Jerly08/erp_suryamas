import React, { useState, useEffect, useRef } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import SimpleLayout from '@/components/layout/SimpleLayout';
import { useAuth } from '@/contexts/AuthContext';
import { useModulePermissions } from '@/hooks/usePermissions';
import { useTranslation } from '@/hooks/useTranslation';
import { useCompanyType } from '@/hooks/useCompanyType';
import {
  Box,
  Button,
  Heading,
  Input,
  InputGroup,
  InputLeftElement,
  Table,
  Tbody,
  Td,
  Th,
  Thead,
  Tr,
  Flex,
  Select,
  useToast,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalCloseButton,
  Image,
  AlertDialog,
  AlertDialogBody,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogContent,
  AlertDialogOverlay,
  useDisclosure,
  Text,
  Grid,
  HStack,
  Tabs,
  TabList,
  Tab,
  TabPanels,
  TabPanel,
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  Icon,
  useColorModeValue,
} from '@chakra-ui/react';
import { FiSearch, FiEdit, FiTrash2, FiUpload, FiEye, FiPlus, FiGrid, FiPackage, FiMapPin, FiSettings, FiChevronDown, FiRefreshCw } from 'react-icons/fi';
import ProductService, { Product, Category, WarehouseLocation } from '@/services/productService';
import ProductForm from './ProductForm';
import CategoryManagement from './CategoryManagement';
import UnitManagement from './UnitManagement';
import WarehouseLocationManagement from './WarehouseLocationManagement';
import { formatIDR, formatCurrencyDetailed } from '@/utils/currency';
import { getProductImageUrl, debugImageUrl } from '@/utils/imageUrl';
import ProductImportModal from './ProductImportModal';

const ProductCatalog: React.FC = () => {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { user } = useAuth();
  const { t } = useTranslation();
  const { isService, hasInventory, getLabel } = useCompanyType();
  const {
    canView,
    canCreate,
    canEdit,
    canDelete,
    canExport
  } = useModulePermissions('products');
  const [products, setProducts] = useState<Product[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [warehouseLocations, setWarehouseLocations] = useState<WarehouseLocation[]>([]);
  const [selectedProduct, setSelectedProduct] = useState<Product | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const [categoryFilter, setCategoryFilter] = useState('');
  const [warehouseLocationFilter, setWarehouseLocationFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [sortBy, setSortBy] = useState('name');
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('asc');
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [pendingUpload, setPendingUpload] = useState<{ productId: number, file: File } | null>(null);
  const [backfillLoadingProductId, setBackfillLoadingProductId] = useState<number | null>(null);
  const { isOpen: isAlertOpen, onOpen: onAlertOpen, onClose: onAlertClose } = useDisclosure();
  const { isOpen: isDetailOpen, onOpen: onDetailOpen, onClose: onDetailClose } = useDisclosure();
  const { isOpen: isManagementModalOpen, onOpen: onManagementModalOpen, onClose: onManagementModalClose } = useDisclosure();
  const { isOpen: isImportModalOpen, onOpen: onImportModalOpen, onClose: onImportModalClose } = useDisclosure();
  const [detailProduct, setDetailProduct] = useState<Product | null>(null);
  const [tabIndex, setTabIndex] = useState(0);
  const [defaultIsAssembly, setDefaultIsAssembly] = useState(false);
  const handledNotificationProductId = useRef<string | null>(null);
  const toast = useToast();
  const pageTitleColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const cardBg = useColorModeValue('white', 'var(--bg-secondary)');
  const subtleBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const tableHeadBg = useColorModeValue('gray.50', 'var(--bg-tertiary)');
  const borderColor = useColorModeValue('gray.200', 'var(--border-color)');
  const textColor = useColorModeValue('gray.700', 'var(--text-primary)');
  const mutedTextColor = useColorModeValue('gray.600', 'var(--text-secondary)');
  const placeholderColor = useColorModeValue('gray.400', 'var(--text-secondary)');
  const searchIconColor = useColorModeValue('gray.400', 'gray.500');
  const inputBg = useColorModeValue('white', 'var(--bg-secondary)');
  const hoverBorderColor = useColorModeValue('gray.300', 'gray.500');
  const rowHoverBg = useColorModeValue('gray.50', 'whiteAlpha.50');
  const stockTextColor = useColorModeValue('gray.800', 'gray.100');
  const noImageTextColor = useColorModeValue('gray.400', 'gray.500');
  const noteBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const imageCardBg = useColorModeValue('white', 'var(--bg-secondary)');
  const labelColor = useColorModeValue('gray.600', 'var(--text-secondary)');
  const sectionTitleColor = useColorModeValue('blue.600', 'blue.200');
  const infoPanelBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const infoPanelTitleColor = useColorModeValue('gray.700', 'gray.100');
  const successColor = useColorModeValue('green.600', 'green.300');
  const warningColor = useColorModeValue('orange.600', 'orange.300');
  const salePriceColor = useColorModeValue('blue.600', 'blue.300');
  const modalBg = useColorModeValue('white', 'var(--bg-secondary)');

  // Pagination states
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [totalItems, setTotalItems] = useState(0);
  const limit = 50; // Default items per page

  // Tooltip descriptions for product page
  const tooltips = {
    search: 'Cari produk berdasarkan nama, kode, atau deskripsi',
    category: 'Kategori produk untuk pengelompokan dan pelaporan',
    unit: 'Satuan unit produk (contoh: Pcs, Kg, Liter, Box)',
    warehouse: 'Lokasi gudang/warehouse tempat produk disimpan',
    stock: 'Jumlah stok tersedia saat ini',
    minStock: 'Stok minimum sebagai peringatan untuk reorder',
    costPrice: 'Harga pokok/cost produk (untuk perhitungan COGS)',
    salePrice: 'Harga jual standar kepada customer',
    productCode: 'Kode unik produk (SKU atau Product Code)',
    barcode: 'Barcode produk untuk scanning',
    description: 'Deskripsi detail produk',
    isActive: 'Status produk: Active (dijual) atau Inactive (tidak dijual)',
    trackInventory: 'Aktifkan tracking inventory untuk produk ini',
    revenueAccount: 'Akun pendapatan di chart of accounts untuk produk ini',
    expenseAccount: 'Akun biaya/expense untuk pembelian produk ini',
  };

  useEffect(() => {
    fetchCategories();
    fetchWarehouseLocations();
  }, []);

  // Reset page to 1 when filters change
  useEffect(() => {
    setCurrentPage(1);
  }, [searchTerm, categoryFilter, warehouseLocationFilter, statusFilter, tabIndex]);

  useEffect(() => {
    fetchProducts();
  }, [searchTerm, categoryFilter, warehouseLocationFilter, statusFilter, sortBy, sortOrder, currentPage, tabIndex]);

  useEffect(() => {
    const notificationProductId = searchParams?.get('notificationProductId');

    if (!notificationProductId || handledNotificationProductId.current === notificationProductId) {
      return;
    }

    handledNotificationProductId.current = notificationProductId;

    const clearNotificationProductParam = () => {
      const nextParams = new URLSearchParams(searchParams?.toString() || '');
      nextParams.delete('notificationProductId');
      const nextUrl = nextParams.toString() ? `/products?${nextParams.toString()}` : '/products';
      router.replace(nextUrl);
    };

    const productId = Number(notificationProductId);
    if (!Number.isFinite(productId)) {
      clearNotificationProductParam();
      return;
    }

    const existingProduct = products.find((product) => product.id === productId);
    if (existingProduct) {
      setDetailProduct(existingProduct);
      onDetailOpen();
      clearNotificationProductParam();
      return;
    }

    const openProductFromNotification = async () => {
      try {
        const response = await ProductService.getProduct(productId);
        setDetailProduct(response?.data || response);
        onDetailOpen();
      } catch (error) {
        toast({
          title: 'Failed to open product from notification',
          status: 'error',
          isClosable: true,
        });
      } finally {
        clearNotificationProductParam();
      }
    };

    void openProductFromNotification();
  }, [onDetailOpen, products, router, searchParams, toast]);

  const fetchProducts = async () => {
    try {
      const params: any = {
        page: currentPage,
        limit,
        is_assembly: tabIndex === 1,
        sort_by: sortBy,
        sort_order: sortOrder,
      };
      if (searchTerm) params.search = searchTerm;
      if (categoryFilter) params.category = categoryFilter;
      if (warehouseLocationFilter) params.warehouse_location = warehouseLocationFilter;
      if (statusFilter) params.status = statusFilter;

      const response = await ProductService.getProducts(params);
      setProducts(response.data || []);
      if (response.meta) {
        setTotalPages(response.meta.total_pages || 1);
        setTotalItems(response.meta.total || 0);
      }
    } catch (error) {
      toast({
        title: 'Failed to fetch products',
        status: 'error',
        isClosable: true,
      });
    }
  };

  const fetchCategories = async () => {
    try {
      const data = await ProductService.getCategories();
      setCategories(data.data);
    } catch (error) {
      toast({
        title: 'Failed to fetch categories',
        status: 'error',
        isClosable: true,
      });
    }
  };

  const fetchWarehouseLocations = async () => {
    try {
      const data = await ProductService.getWarehouseLocations();
      setWarehouseLocations(data.data);

      // Show info message if using mock data
      if (data.message && data.message.includes('mock')) {
        console.info('Using mock warehouse locations data - implement backend API for full functionality');
      }
    } catch (error) {
      console.error('Failed to fetch warehouse locations:', error);
      // Set empty array instead of showing error to user
      setWarehouseLocations([]);
    }
  };

  const handleSearchChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchTerm(e.target.value);
  };

  const handleAddProductClick = () => {
    if (!canCreate) return;
    setSelectedProduct(null);
    setDefaultIsAssembly(tabIndex === 1);
    setIsModalOpen(true);
  };

  const handleSaveProduct = (_product: Product) => {
    setIsModalOpen(false);
    setSelectedProduct(null);
    void fetchProducts();
  };

  const handleCloseModal = () => {
    setIsModalOpen(false);
    setSelectedProduct(null);
  };

  const handleEditProduct = (product: Product) => {
    if (!canEdit) return;
    setSelectedProduct(product);
    setIsModalOpen(true);
  };

  const handleViewDetails = (product: Product) => {
    setDetailProduct(product);
    onDetailOpen();
  };

  const handleDeleteProduct = async (product: Product) => {
    if (!window.confirm(`Are you sure you want to delete "${product.name}"?`)) {
      return;
    }

    try {
      await ProductService.deleteProduct(product.id!);
      void fetchProducts();
      toast({
        title: 'Product deleted successfully',
        status: 'success',
        isClosable: true,
      });
    } catch (error) {
      toast({
        title: 'Failed to delete product',
        status: 'error',
        isClosable: true,
      });
    }
  };

  const handleBackfillFulfillment = async (product: Product) => {
    if (!product.id || backfillLoadingProductId !== null) {
      return;
    }

    if (!window.confirm(`Trigger manual fulfillment backfill untuk "${product.name}"? Stok tersedia akan dialokasikan ke sales backorder tertua.`)) {
      return;
    }

    setBackfillLoadingProductId(product.id);
    try {
      const response = await ProductService.backfillFulfillment(product.id);
      const result = response.data;
      await fetchProducts();
      toast({
        title: result.allocated_quantity > 0 ? 'Backfill fulfillment selesai' : 'Tidak ada backorder yang dialokasikan',
        description: result.allocated_quantity > 0
          ? `${result.allocated_quantity} qty dialokasikan. Sisa stok ${result.remaining_stock}, sisa BO ${result.pending_backorder_quantity}.`
          : `Sisa stok ${result.remaining_stock}, sisa BO ${result.pending_backorder_quantity}.`,
        status: result.allocated_quantity > 0 ? 'success' : 'info',
        isClosable: true,
      });
    } catch (error: any) {
      toast({
        title: 'Backfill fulfillment gagal',
        description: error?.response?.data?.error || error?.message || 'Gagal menjalankan backfill manual',
        status: 'error',
        isClosable: true,
      });
    } finally {
      setBackfillLoadingProductId(null);
    }
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>, productId: number) => {
    if (e.target.files && e.target.files[0]) {
      const file = e.target.files[0];
      const product = products.find(p => p.id === productId);

      if (product && product.image_path) {
        // Product already has an image, show confirmation
        setPendingUpload({ productId, file });
        onAlertOpen();
      } else {
        // No existing image, upload directly
        handleUpload(productId, file);
      }
    }
  };

  const confirmImageUpdate = () => {
    if (pendingUpload) {
      handleUpload(pendingUpload.productId, pendingUpload.file);
      setPendingUpload(null);
    }
    onAlertClose();
  };

  const handleUpload = async (productId: number, file: File) => {
    try {
      const response = await ProductService.uploadProductImage(productId, file);

      // Update the product in the list with the new image path
      setProducts(prevProducts =>
        prevProducts.map(p =>
          p.id === productId
            ? { ...p, image_path: response.path }
            : p
        )
      );

      toast({
        title: 'Image uploaded successfully',
        status: 'success',
        isClosable: true,
      });

      // Reset file input
      const fileInput = document.getElementById(`file-upload-${productId}`) as HTMLInputElement;
      if (fileInput) {
        fileInput.value = '';
      }
    } catch (error) {
      toast({
        title: 'Failed to upload image',
        status: 'error',
        isClosable: true,
      });
    }
  };

  // Management handlers
  const handleOpenManagement = () => {
    if (!canCreate) return;
    onManagementModalOpen();
  };

  return (
    <SimpleLayout>
      <Box>
        <Flex justify="space-between" align="center" mb={6}>
          <Box>
            <Heading as="h1" size="xl" mb={2} color={pageTitleColor}>
              {getLabel('products')} - Katalog Items & Price Benchmarks
            </Heading>
          </Box>

          {/* Management Buttons */}
          <HStack spacing={3}>
            {canCreate && (
              <>
                <Button
                  leftIcon={<FiUpload />}
                  colorScheme="blue"
                  size="lg"
                  onClick={onImportModalOpen}
                  variant="outline"
                >
                  Import
                </Button>
                <Button
                  leftIcon={<FiSettings />}
                  rightIcon={<FiChevronDown />}
                  colorScheme="teal"
                  size="lg"
                  onClick={handleOpenManagement}
                  variant="outline"
                >
                  {t('products.management.title')}
                </Button>
                <Button
                  leftIcon={<FiPlus />}
                  colorScheme="brand"
                  size="lg"
                  onClick={handleAddProductClick}
                >
                  {t('products.addProduct')}
                </Button>
              </>
            )}
          </HStack>
        </Flex>

        {/* Tabs for Component vs Assembly Separation */}
        <Tabs variant="enclosed" colorScheme="blue" index={tabIndex} onChange={(index) => setTabIndex(index)} mb={6}>
          <TabList borderColor={borderColor}>
            <Tab fontWeight="bold">🧱 Komponen & Bahan Baku</Tab>
            <Tab fontWeight="bold">🎯 Master Produk Rakitan</Tab>
          </TabList>
        </Tabs>

        {/* Search and Filters */}
        <Box mb={6}>
          <Flex gap={4} mb={4} flexWrap="wrap">
            {/* Search */}
            <InputGroup maxW="400px">
              <InputLeftElement pointerEvents="none">
                <FiSearch color={searchIconColor} />
              </InputLeftElement>
              <Input
                placeholder={t('products.searchProducts')}
                value={searchTerm}
                onChange={handleSearchChange}
                bg={inputBg}
                color={textColor}
                borderColor={borderColor}
                _hover={{ borderColor: hoverBorderColor }}
                _focusVisible={{ borderColor: 'blue.400', boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)' }}
                _placeholder={{ color: placeholderColor }}
              />
            </InputGroup>

            {/* Category Filter */}
            <Select
              placeholder={t('common.filters.allCategories')}
              value={categoryFilter}
              onChange={(e) => setCategoryFilter(e.target.value)}
              maxW="200px"
              bg={inputBg}
              color={textColor}
              borderColor={borderColor}
              _hover={{ borderColor: hoverBorderColor }}
              _focusVisible={{ borderColor: 'blue.400', boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)' }}
            >
              {categories.map(category => (
                <option key={category.id} value={category.id}>
                  {category.name}
                </option>
              ))}
            </Select>

            {/* Warehouse Location Filter - hidden for SERVICE companies */}
            {hasInventory && (
              <Select
                placeholder={t('common.filters.allLocations')}
                value={warehouseLocationFilter}
                onChange={(e) => setWarehouseLocationFilter(e.target.value)}
                maxW="200px"
                bg={inputBg}
                color={textColor}
                borderColor={borderColor}
                _hover={{ borderColor: hoverBorderColor }}
                _focusVisible={{ borderColor: 'blue.400', boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)' }}
              >
                {warehouseLocations.map(location => (
                  <option key={location.id} value={location.id}>
                    {location.name}
                  </option>
                ))}
              </Select>
            )}

            {/* Status Filter */}
            <Select
              placeholder={t('common.filters.allStatus')}
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              maxW="150px"
              bg={inputBg}
              color={textColor}
              borderColor={borderColor}
              _hover={{ borderColor: hoverBorderColor }}
              _focusVisible={{ borderColor: 'blue.400', boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)' }}
            >
              <option value="active">{t('products.active')}</option>
              <option value="inactive">{t('products.inactive')}</option>
            </Select>

            {/* Sort Options */}
            <Select
              value={`${sortBy}-${sortOrder}`}
              onChange={(e) => {
                const [field, order] = e.target.value.split('-');
                setSortBy(field);
                setSortOrder(order as 'asc' | 'desc');
              }}
              maxW="180px"
              bg={inputBg}
              color={textColor}
              borderColor={borderColor}
              _hover={{ borderColor: hoverBorderColor }}
              _focusVisible={{ borderColor: 'blue.400', boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)' }}
            >
              <option value="name-asc">{t('products.sort.nameAZ')}</option>
              <option value="name-desc">{t('products.sort.nameZA')}</option>
              <option value="code-asc">{t('products.sort.codeAZ')}</option>
              <option value="code-desc">{t('products.sort.codeZA')}</option>
              <option value="category-asc">{t('products.sort.categoryAZ')}</option>
              <option value="stock-desc">{t('products.sort.stockHighLow')}</option>
              <option value="stock-asc">{t('products.sort.stockLowHigh')}</option>
              <option value="sale_price-desc">{t('products.sort.priceHighLow')}</option>
              <option value="sale_price-asc">{t('products.sort.priceLowHigh')}</option>
            </Select>

            {/* Clear Filters */}
            <Button
              onClick={() => {
                setSearchTerm('');
                setCategoryFilter('');
                setWarehouseLocationFilter('');
                setStatusFilter('');
                setSortBy('name');
                setSortOrder('asc');
              }}
              variant="outline"
              size="md"
            >
              {t('common.filters.clearFilters')}
            </Button>
          </Flex>

          {/* Results Summary */}
          <Text fontSize="sm" color={mutedTextColor}>
            {t('products.showing')} {products.length} dari {totalItems} {t('products.product')}
            {(searchTerm || categoryFilter || warehouseLocationFilter || statusFilter) ? ` ${t('products.filtered')}` : ''}
          </Text>
        </Box>

        <Box bg={cardBg} border="1px solid" borderColor={borderColor} borderRadius="lg" overflowX="auto">
          <Table variant="simple">
            <Thead bg={tableHeadBg}>
              <Tr>
                <Th color={mutedTextColor} borderColor={borderColor}>{t('products.table.productId')}</Th>
                <Th color={mutedTextColor} borderColor={borderColor}>{t('products.table.name')}</Th>
                <Th color={mutedTextColor} borderColor={borderColor}>{t('products.table.category')}</Th>
                {hasInventory && <Th color={mutedTextColor} borderColor={borderColor}>{t('products.table.warehouseLocation')}</Th>}
                {hasInventory && <Th color={mutedTextColor} borderColor={borderColor}>Stok</Th>}
                {hasInventory && (
                  <Th color={mutedTextColor} borderColor={borderColor}>{tabIndex === 1 ? 'Harga Pokok / HPP' : 'Acuan Harga Beli'}</Th>
                )}
                {hasInventory && (
                  <Th color={mutedTextColor} borderColor={borderColor}>Nilai Inventory</Th>
                )}
                <Th color={mutedTextColor} borderColor={borderColor}>Acuan Harga Jual</Th>
                <Th color={mutedTextColor} borderColor={borderColor}>{t('products.table.actions')}</Th>
              </Tr>
            </Thead>
            <Tbody>
              {products.map((product) => (
                <Tr key={product.id} _hover={{ bg: rowHoverBg }}>
                  <Td color={textColor} borderColor={borderColor}>{product.code}</Td>
                  <Td color={textColor} borderColor={borderColor}>{product.name}</Td>
                  <Td color={textColor} borderColor={borderColor}>{product.category?.name}</Td>
                  {hasInventory && <Td color={textColor} borderColor={borderColor}>{product.warehouse_location?.name || t('products.table.noLocation')}</Td>}
                  {hasInventory && (
                    <Td borderColor={borderColor}>
                      {product.is_assembly ? (
                        <Text color={mutedTextColor} fontStyle="italic">
                          Tidak Perlu Stok (Auto-Kit)
                        </Text>
                      ) : (
                        <Box>
                          <Text fontWeight="bold" color={product.stock <= (product.min_stock || 0) ? "red.500" : stockTextColor}>
                            {product.stock} {product.unit}
                          </Text>
                          {!!product.pending_backorder_qty && product.pending_backorder_qty > 0 && (
                            <Text fontSize="xs" color="orange.500">
                              BO {product.pending_backorder_qty} | Net {product.net_available_stock ?? product.stock - product.pending_backorder_qty}
                            </Text>
                          )}
                        </Box>
                      )}
                    </Td>
                  )}
                  {hasInventory && (
                    <Td color={tabIndex === 1 ? warningColor : successColor} fontWeight="semibold" borderColor={borderColor}>
                      {formatCurrencyDetailed(tabIndex === 1 ? (product.cost_price || 0) : (product.purchase_price || 0))}
                    </Td>
                  )}
                  {hasInventory && (
                    <Td color={warningColor} fontWeight="semibold" borderColor={borderColor}>
                      {product.is_assembly || product.is_service
                        ? '-'
                        : formatCurrencyDetailed((product.stock || 0) * (product.cost_price || product.purchase_price || 0))}
                    </Td>
                  )}
                  <Td color={salePriceColor} fontWeight="semibold" borderColor={borderColor}>{formatCurrencyDetailed(product.sale_price || 0)}</Td>
                  <Td borderColor={borderColor}>
                    <Button
                      size="sm"
                      variant="ghost"
                      leftIcon={<FiEye />}
                      mr={2}
                      onClick={() => handleViewDetails(product)}
                    >
                      {t('common.view')}
                    </Button>
                    {canEdit && (
                      <>
                        <Button
                          size="sm"
                          variant="ghost"
                          leftIcon={<FiEdit />}
                          mr={2}
                          onClick={() => handleEditProduct(product)}
                        >
                          {t('common.edit')}
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          colorScheme="red"
                          leftIcon={<FiTrash2 />}
                          mr={2}
                          onClick={() => handleDeleteProduct(product)}
                        >
                          {t('common.delete')}
                        </Button>
                        {hasInventory && !product.is_service && !product.is_assembly && (product.pending_backorder_qty || 0) > 0 && (product.stock || 0) > 0 && (
                          <Button
                            size="sm"
                            variant="ghost"
                            colorScheme="orange"
                            leftIcon={<FiRefreshCw />}
                            mr={2}
                            isLoading={backfillLoadingProductId === product.id}
                            loadingText="Backfill"
                            onClick={() => handleBackfillFulfillment(product)}
                          >
                            Backfill BO
                          </Button>
                        )}
                        <Input
                          type="file"
                          accept="image/*"
                          onChange={(e) => handleFileChange(e, product.id!)}
                          style={{ display: 'none' }}
                          id={`file-upload-${product.id}`}
                        />
                        <Button
                          size="sm"
                          variant="ghost"
                          leftIcon={<FiUpload />}
                          as="label"
                          htmlFor={`file-upload-${product.id}`}
                          cursor="pointer"
                        >
                          {product.image_path ? t('products.imageUpload.updateImage') : t('products.imageUpload.uploadImage')}
                        </Button>
                      </>
                    )}
                  </Td>
                </Tr>
              ))}
            </Tbody>
          </Table>
        </Box>

        {/* Pagination Controls */}
        {totalPages > 1 && (
          <Flex justify="space-between" align="center" mt={4} bg={subtleBg} p={3} borderRadius="md" border="1px" borderColor={borderColor}>
            <Text fontSize="sm" color={mutedTextColor}>
              Menampilkan Halaman {currentPage} dari {totalPages} ({totalItems} total data)
            </Text>
            <HStack>
              <Button
                size="sm"
                isDisabled={currentPage === 1}
                onClick={() => setCurrentPage(Math.max(1, currentPage - 1))}
              >
                Previous
              </Button>
              <Button
                size="sm"
                isDisabled={currentPage === totalPages}
                onClick={() => setCurrentPage(currentPage + 1)}
              >
                Next
              </Button>
            </HStack>
          </Flex>
        )}

        {/* Add/Edit Product Modal */}
        {(selectedProduct ? canEdit : canCreate) && (
          <Modal isOpen={isModalOpen || !!selectedProduct} onClose={handleCloseModal} size="6xl">
            <ModalOverlay />
            <ModalContent bg={modalBg} border="1px solid" borderColor={borderColor}>
              <ModalHeader color={textColor} borderBottom="1px solid" borderColor={borderColor}>
                {selectedProduct ? t('products.editProduct') : t('products.addProduct')}
              </ModalHeader>
              <ModalCloseButton />
              <ModalBody pb={6}>
                <ProductForm
                  product={selectedProduct || undefined}
                  defaultIsAssembly={defaultIsAssembly}
                  onSave={handleSaveProduct}
                  onCancel={handleCloseModal}
                />
              </ModalBody>
            </ModalContent>
          </Modal>
        )}

        {/* Product Details Modal */}
        <Modal isOpen={isDetailOpen} onClose={onDetailClose} size="4xl">
          <ModalOverlay />
          <ModalContent bg={modalBg} border="1px solid" borderColor={borderColor}>
            <ModalHeader color={textColor} borderBottom="1px solid" borderColor={borderColor}>
              {t('products.details.title')} - {detailProduct?.name}
            </ModalHeader>
            <ModalCloseButton />
            <ModalBody pb={6}>
              {detailProduct && (
                <Box>
                  {/* Product Image */}
                  <Flex justify="center" mb={6}>
                    {detailProduct.image_path ? (
                      <Image
                        src={getProductImageUrl(detailProduct.image_path) || ''}
                        alt={detailProduct.name}
                        maxH="250px"
                        maxW="350px"
                        objectFit="contain"
                        borderRadius="lg"
                        border="2px"
                        borderColor={borderColor}
                        boxShadow="md"
                        bg={imageCardBg}
                        p={2}
                        fallbackSrc="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='100' height='100' viewBox='0 0 100 100'%3E%3Crect width='100' height='100' fill='%23f0f0f0'/%3E%3Ctext x='50' y='50' text-anchor='middle' dy='.3em' font-family='Arial, sans-serif' font-size='14' fill='%23999'%3ENo Image%3C/text%3E%3C/svg%3E"
                        onError={(e) => {
                          console.error('Image failed to load:', detailProduct.image_path);
                          console.error('Attempted URL:', getProductImageUrl(detailProduct.image_path));
                          debugImageUrl(detailProduct.image_path);
                        }}
                      />
                    ) : (
                      <Box
                        w="350px"
                        h="250px"
                        bg={subtleBg}
                        borderRadius="lg"
                        border="2px"
                        borderColor={borderColor}
                        display="flex"
                        alignItems="center"
                        justifyContent="center"
                        boxShadow="sm"
                      >
                        <Text color={noImageTextColor} fontSize="lg">{t('products.details.noProductImage')}</Text>
                      </Box>
                    )}
                  </Flex>

                  {/* Basic Information */}
                  <Box mb={4}>
                    <Text fontSize="lg" fontWeight="bold" mb={2} color={sectionTitleColor}>{t('products.details.basicInfo')}</Text>
                    <Grid templateColumns="repeat(2, 1fr)" gap={4}>
                      <Box>
                        <Text fontWeight="semibold" color={labelColor}>{t('products.details.productCode')}</Text>
                        <Text fontSize="md" color={textColor}>{detailProduct.code}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color={labelColor}>{t('products.details.productName')}</Text>
                        <Text fontSize="md" color={textColor}>{detailProduct.name}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color={labelColor}>{t('products.details.category')}</Text>
                        <Text fontSize="md" color={textColor}>{detailProduct.category?.name || t('products.details.noCategory')}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color={labelColor}>{t('products.details.unit')}</Text>
                        <Text fontSize="md" color={textColor}>{detailProduct.unit}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color={labelColor}>{t('products.details.warehouseLocation')}</Text>
                        <Text fontSize="md" color={textColor}>{detailProduct.warehouse_location?.name || t('products.details.noLocationAssigned')}</Text>
                      </Box>
                    </Grid>
                    {detailProduct.description && (
                      <Box mt={3}>
                        <Text fontWeight="semibold" color={labelColor}>{t('products.table.description')}</Text>
                        <Text fontSize="md" color={textColor}>{detailProduct.description}</Text>
                      </Box>
                    )}
                  </Box>

                  {/* Product Details */}
                  <Box mb={4}>
                    <Text fontSize="lg" fontWeight="bold" mb={2} color={sectionTitleColor}>{t('products.details.productDetails')}</Text>
                    <Grid templateColumns="repeat(2, 1fr)" gap={4}>
                      <Box>
                        <Text fontWeight="semibold" color={labelColor}>{t('products.details.brand')}</Text>
                        <Text fontSize="md" color={textColor}>{detailProduct.brand || t('products.details.notSpecified')}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color={labelColor}>{t('products.details.model')}</Text>
                        <Text fontSize="md" color={textColor}>{detailProduct.model || t('products.details.notSpecified')}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color={labelColor}>{t('products.details.sku')}</Text>
                        <Text fontSize="md" color={textColor}>{detailProduct.sku || t('products.details.notSpecified')}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color={labelColor}>{t('products.details.barcode')}</Text>
                        <Text fontSize="md" color={textColor}>{detailProduct.barcode || t('products.details.notSpecified')}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color={labelColor}>{t('products.details.weight')}</Text>
                        <Text fontSize="md" color={textColor}>{detailProduct.weight} kg</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color={labelColor}>{t('products.details.dimensions')}</Text>
                        <Text fontSize="md" color={textColor}>{detailProduct.dimensions || t('products.details.notSpecified')}</Text>
                      </Box>
                    </Grid>
                  </Box>

                  {/* Pricing & Margin Analysis */}
                  <Box mb={4}>
                    <Text fontSize="lg" fontWeight="bold" mb={2} color={sectionTitleColor}>{t('products.details.pricing')}</Text>
                    <Grid templateColumns={hasInventory ? 'repeat(4, 1fr)' : 'repeat(2, 1fr)'} gap={4}>
                      {/* Purchase Price - hidden for SERVICE companies */}
                      {hasInventory && (
                        <Box>
                          <Text fontWeight="semibold" color={labelColor}>{t('products.details.purchasePrice')}</Text>
                          <Text fontSize="md" color={successColor} fontWeight="bold">
                            {formatCurrencyDetailed(detailProduct.purchase_price || 0)}
                          </Text>
                        </Box>
                      )}
                      {/* Cost Price / HPP */}
                      {hasInventory && (
                        <Box>
                          <Text fontWeight="semibold" color={labelColor}>Harga Pokok (HPP/COGS)</Text>
                          <Text fontSize="md" color={warningColor} fontWeight="bold">
                            {formatCurrencyDetailed(detailProduct.cost_price || 0)}
                          </Text>
                          <Text fontSize="xs" color={mutedTextColor}>
                            Termasuk alokasi freight dari purchase.
                          </Text>
                        </Box>
                      )}
                      <Box>
                        <Text fontWeight="semibold" color={labelColor}>{t('products.details.salePrice')}</Text>
                        <Text fontSize="md" color={salePriceColor} fontWeight="bold">
                          {formatCurrencyDetailed(detailProduct.sale_price || 0)}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color={labelColor}>{t('products.details.pricingTier')}</Text>
                        <Text fontSize="md" color={textColor}>{detailProduct.pricing_tier || t('products.details.standard')}</Text>
                      </Box>
                    </Grid>

                    {/* Margin Analysis - only for inventory companies with valid prices */}
                    {hasInventory && (detailProduct.cost_price || 0) > 0 && (detailProduct.sale_price || 0) > 0 && (
                      <Box mt={4} p={3} bg={infoPanelBg} borderRadius="md" border="1px" borderColor={borderColor}>
                        <Text fontWeight="bold" color={infoPanelTitleColor} mb={2}>📊 Analisis Margin</Text>
                        <Grid templateColumns="repeat(3, 1fr)" gap={4}>
                          <Box>
                            <Text fontSize="sm" color={mutedTextColor}>Laba Kotor per Unit</Text>
                            <Text fontSize="md" fontWeight="bold" color={
                              (detailProduct.sale_price || 0) - (detailProduct.cost_price || 0) > 0 ? 'green.600' : 'red.600'
                            }>
                              {formatCurrencyDetailed((detailProduct.sale_price || 0) - (detailProduct.cost_price || 0))}
                            </Text>
                          </Box>
                          <Box>
                            <Text fontSize="sm" color={mutedTextColor}>Gross Margin</Text>
                            <Text fontSize="md" fontWeight="bold" color={
                              ((detailProduct.sale_price || 0) - (detailProduct.cost_price || 0)) / (detailProduct.sale_price || 1) * 100 > 0
                                ? 'green.600' : 'red.600'
                            }>
                              {(((detailProduct.sale_price || 0) - (detailProduct.cost_price || 0)) / (detailProduct.sale_price || 1) * 100).toFixed(1)}%
                            </Text>
                          </Box>
                          <Box>
                            <Text fontSize="sm" color={mutedTextColor}>Markup</Text>
                            <Text fontSize="md" fontWeight="bold" color={
                              ((detailProduct.sale_price || 0) - (detailProduct.cost_price || 0)) / (detailProduct.cost_price || 1) * 100 > 0
                                ? 'green.600' : 'red.600'
                            }>
                              {(((detailProduct.sale_price || 0) - (detailProduct.cost_price || 0)) / (detailProduct.cost_price || 1) * 100).toFixed(1)}%
                            </Text>
                          </Box>
                        </Grid>
                      </Box>
                    )}
                  </Box>

                  {/* Inventory - hidden for SERVICE companies */}
                  {hasInventory && (
                    <Box mb={4}>
                      <Text fontSize="lg" fontWeight="bold" mb={2} color={sectionTitleColor}>{t('products.details.inventory')}</Text>
                      <Grid templateColumns="repeat(6, 1fr)" gap={4}>
                        <Box>
                          <Text fontWeight="semibold" color={labelColor}>{t('products.details.currentStock')}</Text>
                          <Text fontSize="md" fontWeight="bold" color={textColor}>{detailProduct.stock}</Text>
                        </Box>
                        <Box>
                          <Text fontWeight="semibold" color={labelColor}>Pending Backorder</Text>
                          <Text fontSize="md" color={detailProduct.pending_backorder_qty ? 'orange.500' : textColor}>
                            {detailProduct.pending_backorder_qty || 0}
                          </Text>
                        </Box>
                        <Box>
                          <Text fontWeight="semibold" color={labelColor}>Net Available</Text>
                          <Text fontSize="md" fontWeight="bold" color={textColor}>
                            {detailProduct.net_available_stock ?? detailProduct.stock}
                          </Text>
                        </Box>
                        <Box>
                          <Text fontWeight="semibold" color={labelColor}>Nilai Inventory</Text>
                          <Text fontSize="md" fontWeight="bold" color={warningColor}>
                            {formatCurrencyDetailed((detailProduct.stock || 0) * (detailProduct.cost_price || detailProduct.purchase_price || 0))}
                          </Text>
                        </Box>
                        <Box>
                          <Text fontWeight="semibold" color={labelColor}>{t('products.details.minStock')}</Text>
                          <Text fontSize="md" color={textColor}>{detailProduct.min_stock}</Text>
                        </Box>
                        <Box>
                          <Text fontWeight="semibold" color={labelColor}>{t('products.details.maxStock')}</Text>
                          <Text fontSize="md" color={textColor}>{detailProduct.max_stock}</Text>
                        </Box>
                        <Box>
                          <Text fontWeight="semibold" color={labelColor}>{t('products.details.reorderLevel')}</Text>
                          <Text fontSize="md" color={textColor}>{detailProduct.reorder_level}</Text>
                        </Box>
                        <Box>
                          <Text fontWeight="semibold" color={labelColor}>Open Orders</Text>
                          <Text fontSize="md" color={textColor}>{detailProduct.backorder_order_count || 0}</Text>
                        </Box>
                      </Grid>
                    </Box>
                  )}

                  {/* Settings */}
                  <Box mb={4}>
                    <Text fontSize="lg" fontWeight="bold" mb={2} color={sectionTitleColor}>{t('products.details.settings')}</Text>
                    <Grid templateColumns="repeat(2, 1fr)" gap={4}>
                      <Box>
                        <Text fontWeight="semibold" color={labelColor}>{t('products.details.statusLabel')}</Text>
                        <Text fontSize="md" color={detailProduct.is_active ? 'green.600' : 'red.600'}>
                          {detailProduct.is_active ? t('products.active') : t('products.inactive')}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color={labelColor}>{t('products.details.serviceProduct')}</Text>
                        <Text fontSize="md" color={textColor}>{detailProduct.is_service ? t('products.details.yes') : t('products.details.no')}</Text>
                      </Box>
                    </Grid>
                  </Box>

                  {/* Notes */}
                  {detailProduct.notes && (
                    <Box>
                      <Text fontSize="lg" fontWeight="bold" mb={2} color={sectionTitleColor}>{t('products.details.notes')}</Text>
                      <Text fontSize="md" p={3} bg={noteBg} borderRadius="md" color={textColor}>
                        {detailProduct.notes}
                      </Text>
                    </Box>
                  )}
                </Box>
              )}
            </ModalBody>
          </ModalContent>
        </Modal>

        {/* Management Modal with Tabs */}
        {canCreate && (
          <Modal isOpen={isManagementModalOpen} onClose={onManagementModalClose} size="6xl">
            <ModalOverlay />
            <ModalContent maxH="90vh" bg={modalBg} border="1px solid" borderColor={borderColor}>
              <ModalHeader color={textColor} borderBottom="1px solid" borderColor={borderColor}>
                {t('products.management.title')}
              </ModalHeader>
              <ModalCloseButton />
              <ModalBody pb={6} overflowY="auto">
                <Tabs colorScheme="teal" variant="enclosed">
                  <TabList>
                    <Tab>
                      <Icon as={FiGrid} mr={2} />
                      {t('products.management.categories')}
                    </Tab>
                    <Tab>
                      <Icon as={FiPackage} mr={2} />
                      {t('products.management.units')}
                    </Tab>
                    <Tab>
                      <Icon as={FiMapPin} mr={2} />
                      {t('products.management.warehouseLocations')}
                    </Tab>
                  </TabList>

                  <TabPanels>
                    <TabPanel>
                      <CategoryManagement />
                    </TabPanel>
                    <TabPanel>
                      <UnitManagement />
                    </TabPanel>
                    <TabPanel>
                      <WarehouseLocationManagement />
                    </TabPanel>
                  </TabPanels>
                </Tabs>
              </ModalBody>
            </ModalContent>
          </Modal>
        )}

        {/* Image Update Confirmation Dialog */}
        <AlertDialog
          isOpen={isAlertOpen}
          leastDestructiveRef={React.useRef(null)}
          onClose={onAlertClose}
        >
          <AlertDialogOverlay>
            <AlertDialogContent bg={modalBg} border="1px solid" borderColor={borderColor}>
              <AlertDialogHeader fontSize="lg" fontWeight="bold" color={textColor}>
                {t('products.imageUpload.updateProductImage')}
              </AlertDialogHeader>

              <AlertDialogBody>
                {t('products.imageUpload.confirmReplace')}
              </AlertDialogBody>

              <AlertDialogFooter>
                <Button onClick={onAlertClose}>
                  {t('common.cancel')}
                </Button>
                <Button colorScheme="blue" onClick={confirmImageUpdate} ml={3}>
                  {t('products.imageUpload.updateImage')}
                </Button>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialogOverlay>
        </AlertDialog>

        {/* Product Import Modal */}
        {canCreate && (
          <ProductImportModal
            isOpen={isImportModalOpen}
            onClose={onImportModalClose}
            onSuccess={fetchProducts}
            categories={categories}
          />
        )}
      </Box>
    </SimpleLayout>
  );
};

export default ProductCatalog;

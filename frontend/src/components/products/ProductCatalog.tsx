import React, { useState, useEffect, useMemo } from 'react';
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
  Icon
} from '@chakra-ui/react';
import { FiSearch, FiEdit, FiTrash2, FiUpload, FiEye, FiPlus, FiGrid, FiPackage, FiMapPin, FiSettings, FiChevronDown } from 'react-icons/fi';
import ProductService, { Product, Category, WarehouseLocation } from '@/services/productService';
import ProductForm from './ProductForm';
import CategoryManagement from './CategoryManagement';
import UnitManagement from './UnitManagement';
import WarehouseLocationManagement from './WarehouseLocationManagement';
import { ProductUnit } from './UnitForm';
import { formatIDR, formatCurrencyDetailed } from '@/utils/currency';
import { getProductImageUrl, debugImageUrl } from '@/utils/imageUrl';

const ProductCatalog: React.FC = () => {
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
  const { isOpen: isAlertOpen, onOpen: onAlertOpen, onClose: onAlertClose } = useDisclosure();
  const { isOpen: isDetailOpen, onOpen: onDetailOpen, onClose: onDetailClose } = useDisclosure();
  const { isOpen: isManagementModalOpen, onOpen: onManagementModalOpen, onClose: onManagementModalClose } = useDisclosure();
  const [detailProduct, setDetailProduct] = useState<Product | null>(null);
  const toast = useToast();

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
    fetchProducts();
    fetchCategories();
    fetchWarehouseLocations();
  }, []);

  useEffect(() => {
    fetchProducts();
  }, [searchTerm, categoryFilter, warehouseLocationFilter, statusFilter]);

  const fetchProducts = async () => {
    try {
      const params: any = {};
      if (searchTerm) params.search = searchTerm;
      if (categoryFilter) params.category = categoryFilter;

      const data = await ProductService.getProducts(params);
      setProducts(data.data);
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

  const handleSearch = () => {
    fetchProducts();
  };

  const handleAddProductClick = () => {
    if (!canCreate) return;
    setSelectedProduct(null);
    setIsModalOpen(true);
  };

  const handleSaveProduct = (product: Product) => {
    if (selectedProduct) {
      // Update existing product in list
      setProducts(prevProducts =>
        prevProducts.map(p => p.id === product.id ? product : p)
      );
    } else {
      // Add new product to list
      setProducts(prevProducts => [...prevProducts, product]);
    }
    setIsModalOpen(false);
    setSelectedProduct(null);
  };

  const handleCloseModal = () => {
    setIsModalOpen(false);
    setSelectedProduct(null);
  };

  const handleEditProduct = (product: Product) => {
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
      setProducts(prevProducts => prevProducts.filter(p => p.id !== product.id));
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

  // Filtered and sorted products using useMemo for performance

  const filteredAndSortedProducts = useMemo(() => {
    return products
      .filter(product => {
        const matchesSearch = searchTerm ?
          product.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
          product.code.toLowerCase().includes(searchTerm.toLowerCase()) : true;
        const matchesCategory = categoryFilter ?
          product.category?.id === Number(categoryFilter) : true;
        const matchesWarehouseLocation = warehouseLocationFilter ?
          product.warehouse_location?.id === Number(warehouseLocationFilter) : true;
        const matchesStatus = statusFilter ?
          (statusFilter === 'active' ? product.is_active : !product.is_active) : true;
        return matchesSearch && matchesCategory && matchesWarehouseLocation && matchesStatus;
      })
      .sort((a, b) => {
        let comparison = 0;
        if (sortBy === 'name' || sortBy === 'code' || sortBy === 'category') {
          const aValue = sortBy === 'category' ? a.category?.name || '' : a[sortBy as keyof Product] as string;
          const bValue = sortBy === 'category' ? b.category?.name || '' : b[sortBy as keyof Product] as string;
          comparison = (aValue < bValue ? -1 : (aValue > bValue ? 1 : 0)) * (sortOrder === 'asc' ? 1 : -1);
        } else if (sortBy === 'stock' || sortBy === 'sale_price') {
          const aValue = a[sortBy as keyof Product] as number;
          const bValue = b[sortBy as keyof Product] as number;
          comparison = (aValue < bValue ? -1 : (aValue > bValue ? 1 : 0)) * (sortOrder === 'asc' ? 1 : -1);
        }
        return comparison;
      });
  }, [products, searchTerm, categoryFilter, warehouseLocationFilter, statusFilter, sortBy, sortOrder]);

  return (
    <SimpleLayout allowedRoles={['admin', 'inventory_manager', 'employee', 'finance', 'director']}>
      <Box>
        <Flex justify="space-between" align="center" mb={6}>
          <Box>
            <Heading as="h1" size="xl" mb={2}>{getLabel('products')} - Katalog Items & Price Benchmarks</Heading>
          </Box>

          {/* Management Buttons */}
          <HStack spacing={3}>
            {canCreate && (
              <>
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

        {/* Search and Filters */}
        <Box mb={6}>
          <Flex gap={4} mb={4} flexWrap="wrap">
            {/* Search */}
            <InputGroup maxW="400px">
              <InputLeftElement pointerEvents="none">
                <FiSearch color="gray.300" />
              </InputLeftElement>
              <Input
                placeholder={t('products.searchProducts')}
                value={searchTerm}
                onChange={handleSearchChange}
              />
            </InputGroup>

            {/* Category Filter */}
            <Select
              placeholder={t('common.filters.allCategories')}
              value={categoryFilter}
              onChange={(e) => setCategoryFilter(e.target.value)}
              maxW="200px"
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
          <Text fontSize="sm" color="gray.600">
            {t('products.showing')} {filteredAndSortedProducts.length} {filteredAndSortedProducts.length !== 1 ? t('products.productsPlural') : t('products.product')}
            {(searchTerm || categoryFilter || warehouseLocationFilter || statusFilter) ? ` ${t('products.filtered')}` : ''}
          </Text>
        </Box>

        <Table variant="simple">
          <Thead>
            <Tr>
              <Th>{t('products.table.productId')}</Th>
              <Th>{t('products.table.name')}</Th>
              <Th>{t('products.table.category')}</Th>
              {hasInventory && <Th>{t('products.table.warehouseLocation')}</Th>}
              {hasInventory && <Th>Acuan Harga Beli</Th>}
              <Th>Acuan Harga Jual</Th>
              <Th>{t('products.table.actions')}</Th>
            </Tr>
          </Thead>
          <Tbody>
            {filteredAndSortedProducts.map((product) => (
              <Tr key={product.id}>
                <Td>{product.code}</Td>
                <Td>{product.name}</Td>
                <Td>{product.category?.name}</Td>
                {hasInventory && <Td>{product.warehouse_location?.name || t('products.table.noLocation')}</Td>}
                {hasInventory && <Td color="green.600" fontWeight="semibold">{formatCurrencyDetailed(product.purchase_price || 0)}</Td>}
                <Td color="blue.600" fontWeight="semibold">{formatCurrencyDetailed(product.sale_price || 0)}</Td>
                <Td>
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

        {/* Add/Edit Product Modal */}
        {canEdit && (
          <Modal isOpen={isModalOpen || !!selectedProduct} onClose={handleCloseModal} size="6xl">
            <ModalOverlay />
            <ModalContent>
              <ModalHeader>
                {selectedProduct ? t('products.editProduct') : t('products.addProduct')}
              </ModalHeader>
              <ModalCloseButton />
              <ModalBody pb={6}>
                <ProductForm
                  product={selectedProduct || undefined}
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
          <ModalContent>
            <ModalHeader>
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
                        borderColor="gray.300"
                        boxShadow="md"
                        bg="white"
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
                        bg="gray.50"
                        borderRadius="lg"
                        border="2px"
                        borderColor="gray.200"
                        display="flex"
                        alignItems="center"
                        justifyContent="center"
                        boxShadow="sm"
                      >
                        <Text color="gray.400" fontSize="lg">{t('products.details.noProductImage')}</Text>
                      </Box>
                    )}
                  </Flex>

                  {/* Basic Information */}
                  <Box mb={4}>
                    <Text fontSize="lg" fontWeight="bold" mb={2} color="blue.600">{t('products.details.basicInfo')}</Text>
                    <Grid templateColumns="repeat(2, 1fr)" gap={4}>
                      <Box>
                        <Text fontWeight="semibold" color="gray.600">{t('products.details.productCode')}</Text>
                        <Text fontSize="md">{detailProduct.code}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color="gray.600">{t('products.details.productName')}</Text>
                        <Text fontSize="md">{detailProduct.name}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color="gray.600">{t('products.details.category')}</Text>
                        <Text fontSize="md">{detailProduct.category?.name || t('products.details.noCategory')}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color="gray.600">{t('products.details.unit')}</Text>
                        <Text fontSize="md">{detailProduct.unit}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color="gray.600">{t('products.details.warehouseLocation')}</Text>
                        <Text fontSize="md">{detailProduct.warehouse_location?.name || t('products.details.noLocationAssigned')}</Text>
                      </Box>
                    </Grid>
                    {detailProduct.description && (
                      <Box mt={3}>
                        <Text fontWeight="semibold" color="gray.600">{t('products.table.description')}</Text>
                        <Text fontSize="md">{detailProduct.description}</Text>
                      </Box>
                    )}
                  </Box>

                  {/* Product Details */}
                  <Box mb={4}>
                    <Text fontSize="lg" fontWeight="bold" mb={2} color="blue.600">{t('products.details.productDetails')}</Text>
                    <Grid templateColumns="repeat(2, 1fr)" gap={4}>
                      <Box>
                        <Text fontWeight="semibold" color="gray.600">{t('products.details.brand')}</Text>
                        <Text fontSize="md">{detailProduct.brand || t('products.details.notSpecified')}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color="gray.600">{t('products.details.model')}</Text>
                        <Text fontSize="md">{detailProduct.model || t('products.details.notSpecified')}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color="gray.600">{t('products.details.sku')}</Text>
                        <Text fontSize="md">{detailProduct.sku || t('products.details.notSpecified')}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color="gray.600">{t('products.details.barcode')}</Text>
                        <Text fontSize="md">{detailProduct.barcode || t('products.details.notSpecified')}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color="gray.600">{t('products.details.weight')}</Text>
                        <Text fontSize="md">{detailProduct.weight} kg</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color="gray.600">{t('products.details.dimensions')}</Text>
                        <Text fontSize="md">{detailProduct.dimensions || t('products.details.notSpecified')}</Text>
                      </Box>
                    </Grid>
                  </Box>

                  {/* Pricing & Margin Analysis */}
                  <Box mb={4}>
                    <Text fontSize="lg" fontWeight="bold" mb={2} color="blue.600">{t('products.details.pricing')}</Text>
                    <Grid templateColumns={hasInventory ? 'repeat(4, 1fr)' : 'repeat(2, 1fr)'} gap={4}>
                      {/* Purchase Price - hidden for SERVICE companies */}
                      {hasInventory && (
                        <Box>
                          <Text fontWeight="semibold" color="gray.600">{t('products.details.purchasePrice')}</Text>
                          <Text fontSize="md" color="green.600" fontWeight="bold">
                            {formatCurrencyDetailed(detailProduct.purchase_price || 0)}
                          </Text>
                        </Box>
                      )}
                      {/* Cost Price / HPP */}
                      {hasInventory && (
                        <Box>
                          <Text fontWeight="semibold" color="gray.600">Harga Pokok (HPP/COGS)</Text>
                          <Text fontSize="md" color="orange.600" fontWeight="bold">
                            {formatCurrencyDetailed(detailProduct.cost_price || 0)}
                          </Text>
                        </Box>
                      )}
                      <Box>
                        <Text fontWeight="semibold" color="gray.600">{t('products.details.salePrice')}</Text>
                        <Text fontSize="md" color="blue.600" fontWeight="bold">
                          {formatCurrencyDetailed(detailProduct.sale_price || 0)}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color="gray.600">{t('products.details.pricingTier')}</Text>
                        <Text fontSize="md">{detailProduct.pricing_tier || t('products.details.standard')}</Text>
                      </Box>
                    </Grid>

                    {/* Margin Analysis - only for inventory companies with valid prices */}
                    {hasInventory && (detailProduct.cost_price || 0) > 0 && (detailProduct.sale_price || 0) > 0 && (
                      <Box mt={4} p={3} bg="gray.50" borderRadius="md" border="1px" borderColor="gray.200">
                        <Text fontWeight="bold" color="gray.700" mb={2}>📊 Analisis Margin</Text>
                        <Grid templateColumns="repeat(3, 1fr)" gap={4}>
                          <Box>
                            <Text fontSize="sm" color="gray.500">Laba Kotor per Unit</Text>
                            <Text fontSize="md" fontWeight="bold" color={
                              (detailProduct.sale_price || 0) - (detailProduct.cost_price || 0) > 0 ? 'green.600' : 'red.600'
                            }>
                              {formatCurrencyDetailed((detailProduct.sale_price || 0) - (detailProduct.cost_price || 0))}
                            </Text>
                          </Box>
                          <Box>
                            <Text fontSize="sm" color="gray.500">Gross Margin</Text>
                            <Text fontSize="md" fontWeight="bold" color={
                              ((detailProduct.sale_price || 0) - (detailProduct.cost_price || 0)) / (detailProduct.sale_price || 1) * 100 > 0
                                ? 'green.600' : 'red.600'
                            }>
                              {(((detailProduct.sale_price || 0) - (detailProduct.cost_price || 0)) / (detailProduct.sale_price || 1) * 100).toFixed(1)}%
                            </Text>
                          </Box>
                          <Box>
                            <Text fontSize="sm" color="gray.500">Markup</Text>
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
                      <Text fontSize="lg" fontWeight="bold" mb={2} color="blue.600">{t('products.details.inventory')}</Text>
                      <Grid templateColumns="repeat(4, 1fr)" gap={4}>
                        <Box>
                          <Text fontWeight="semibold" color="gray.600">{t('products.details.currentStock')}</Text>
                          <Text fontSize="md" fontWeight="bold">{detailProduct.stock}</Text>
                        </Box>
                        <Box>
                          <Text fontWeight="semibold" color="gray.600">{t('products.details.minStock')}</Text>
                          <Text fontSize="md">{detailProduct.min_stock}</Text>
                        </Box>
                        <Box>
                          <Text fontWeight="semibold" color="gray.600">{t('products.details.maxStock')}</Text>
                          <Text fontSize="md">{detailProduct.max_stock}</Text>
                        </Box>
                        <Box>
                          <Text fontWeight="semibold" color="gray.600">{t('products.details.reorderLevel')}</Text>
                          <Text fontSize="md">{detailProduct.reorder_level}</Text>
                        </Box>
                      </Grid>
                    </Box>
                  )}

                  {/* Settings */}
                  <Box mb={4}>
                    <Text fontSize="lg" fontWeight="bold" mb={2} color="blue.600">{t('products.details.settings')}</Text>
                    <Grid templateColumns="repeat(3, 1fr)" gap={4}>
                      <Box>
                        <Text fontWeight="semibold" color="gray.600">{t('products.details.statusLabel')}</Text>
                        <Text fontSize="md" color={detailProduct.is_active ? 'green.600' : 'red.600'}>
                          {detailProduct.is_active ? t('products.active') : t('products.inactive')}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color="gray.600">{t('products.details.serviceProduct')}</Text>
                        <Text fontSize="md">{detailProduct.is_service ? t('products.details.yes') : t('products.details.no')}</Text>
                      </Box>
                      <Box>
                        <Text fontWeight="semibold" color="gray.600">{t('products.details.taxable')}</Text>
                        <Text fontSize="md">{detailProduct.taxable ? t('products.details.yes') : t('products.details.no')}</Text>
                      </Box>
                    </Grid>
                  </Box>

                  {/* Notes */}
                  {detailProduct.notes && (
                    <Box>
                      <Text fontSize="lg" fontWeight="bold" mb={2} color="blue.600">{t('products.details.notes')}</Text>
                      <Text fontSize="md" p={3} bg="gray.50" borderRadius="md">
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
            <ModalContent maxH="90vh">
              <ModalHeader>
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
            <AlertDialogContent>
              <AlertDialogHeader fontSize="lg" fontWeight="bold">
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
      </Box>
    </SimpleLayout>
  );
};

export default ProductCatalog;

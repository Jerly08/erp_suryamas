import React, { useState, useEffect } from 'react';
import {
  Box,
  Button,
  FormControl,
  FormLabel,
  Input,
  Select,
  Textarea,
  Grid,
  GridItem,
  Divider,
  Switch,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  useToast,
  VStack,
  HStack,
  Text,
  Image,
  AspectRatio,
  IconButton,
  SimpleGrid
} from '@chakra-ui/react';
import { FiSave, FiX, FiUpload, FiTrash2 } from 'react-icons/fi';
import ProductService, { Product, Category, ProductUnit, WarehouseLocation } from '@/services/productService';
import CurrencyInput from '@/components/common/CurrencyInput';
import { getProductImageUrl, debugImageUrl } from '@/utils/imageUrl';
import { useTranslation } from '@/hooks/useTranslation';

interface ProductFormProps {
  product?: Product;
  defaultIsAssembly?: boolean;
  onSave: (product: Product) => void;
  onCancel: () => void;
}

const ProductForm: React.FC<ProductFormProps> = ({ product, defaultIsAssembly, onSave, onCancel }) => {
  const { t } = useTranslation();
  const [formData, setFormData] = useState<Partial<Product>>({
    code: '',
    name: '',
    description: '',
    category_id: undefined,
    warehouse_location_id: undefined,
    brand: '',
    model: '',
    unit: '',
    purchase_price: 0,
    cost_price: 0, // ✅ ADDED: Harga Pokok
    sale_price: 0,
    pricing_tier: '',
    stock: 0,
    min_stock: 0,
    max_stock: 0,
    reorder_level: 0,
    barcode: '',
    sku: '',
    weight: 0,
    dimensions: '',
    is_active: true,
    is_service: false,
    is_assembly: false,
    taxable: true,
    image_path: '',
    notes: '',
  });

  const [categories, setCategories] = useState<Category[]>([]);
  const [units, setUnits] = useState<ProductUnit[]>([]);
  const [warehouseLocations, setWarehouseLocations] = useState<WarehouseLocation[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [selectedImage, setSelectedImage] = useState<File | null>(null);
  const [imagePreview, setImagePreview] = useState<string | null>(null);
  const toast = useToast();

  useEffect(() => {
    fetchCategories();
    fetchUnits();
    fetchWarehouseLocations();
    if (product) {
      setFormData(product);
    } else if (defaultIsAssembly) {
      setFormData(prev => ({ ...prev, is_assembly: true, purchase_price: 0, cost_price: 0 }));
    }
  }, [product, defaultIsAssembly]);

  const fetchCategories = async () => {
    try {
      const data = await ProductService.getCategories();
      setCategories(data.data);
    } catch (error) {
      toast({
        title: t('products.messages.fetchCategoriesFailed'),
        status: 'error',
        isClosable: true,
      });
    }
  };

  const fetchUnits = async () => {
    try {
      const data = await ProductService.getProductUnits();
      setUnits(data.data);
    } catch (error) {
      toast({
        title: t('products.messages.fetchUnitsFailed'),
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

  const handleInputChange = (field: keyof Product, value: any) => {
    if (field === 'is_assembly') {
      setFormData(prev => ({
        ...prev,
        is_assembly: value,
        purchase_price: value ? 0 : prev.purchase_price,
        cost_price: value ? 0 : prev.cost_price,
        is_service: value ? false : prev.is_service,
      }));
      return;
    }
    setFormData(prev => ({
      ...prev,
      [field]: value,
    }));
  };

  const handleImageChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      const file = e.target.files[0];
      setSelectedImage(file);

      // Create preview URL
      const reader = new FileReader();
      reader.onload = (e) => {
        setImagePreview(e.target?.result as string);
      };
      reader.readAsDataURL(file);
    }
  };

  const handleImageUpload = async () => {
    if (!selectedImage || !formData.id) {
      toast({
        title: t('products.form.saveProductFirst'),
        status: 'warning',
        isClosable: true,
      });
      return;
    }

    try {
      const response = await ProductService.uploadProductImage(formData.id, selectedImage);
      handleInputChange('image_path', response.path);
      setSelectedImage(null);
      setImagePreview(null);

      toast({
        title: t('products.messages.imageUploadSuccess'),
        status: 'success',
        isClosable: true,
      });
    } catch (error: any) {
      console.error('Image upload error:', error);
      let errorMessage = t('products.messages.imageUploadFailed');
      let errorDetail = t('messages.toast.unknownErrorDesc');

      if (error.response?.data?.error) {
        errorDetail = error.response.data.error;
      } else if (error.message) {
        errorDetail = error.message;
      }

      toast({
        title: errorMessage,
        description: errorDetail,
        status: 'error',
        isClosable: true,
        duration: 5000,
      });
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);

    try {
      let result;
      // Copy formData to avoid mutating state
      const submitData = { ...formData };

      // Sanitasi nilai gudang jika produk adalah Jasa
      if (submitData.is_service) {
        submitData.warehouse_location_id = undefined;
      }

      if (product?.id) {
        result = await ProductService.updateProduct(product.id, submitData);
      } else {
        result = await ProductService.createProduct(submitData as Product);
      }

      toast({
        title: product?.id ? t('products.messages.updateSuccess') : t('products.messages.createSuccess'),
        status: 'success',
        isClosable: true,
      });

      onSave(result.data);
    } catch (error: any) {
      toast({
        title: product?.id ? t('products.messages.updateFailed') : t('products.messages.createFailed'),
        description: error.response?.data?.error || t('messages.toast.unknownErrorDesc'),
        status: 'error',
        isClosable: true,
      });
    } finally {
      setIsLoading(false);
    }
  };

  const pricingTiers = [
    'Standard', 'Premium', 'VIP', 'Wholesale', 'Retail'
  ];

  return (
    <Box as="form" onSubmit={handleSubmit}>
      <VStack spacing={6} align="stretch">
        {/* Basic Information */}
        <Box>
          <Text fontSize="lg" fontWeight="bold" mb={4}>{t('products.form.basicInfo')}</Text>

          {/* Product Status & Type - Consolidated at top */}
          <Box mb={6} bg="blue.50" p={4} borderRadius="md">
            <VStack align="stretch" spacing={4}>
              <FormControl display="flex" alignItems="center">
                <FormLabel htmlFor="is_service" mb="0" fontWeight="bold" flex="1">
                  {t('products.form.isService')}
                  <Text fontSize="sm" fontWeight="normal" color="gray.600">
                    {formData.is_service ? "Produk jasa tidak memerlukan stok dan pengiriman" : "Produk fisik dengan stok dan inventaris"}
                  </Text>
                </FormLabel>
                <Switch
                  id="is_service"
                  isChecked={formData.is_service}
                  onChange={(e) => {
                    handleInputChange('is_service', e.target.checked);
                    if (e.target.checked && formData.is_assembly) {
                      handleInputChange('is_assembly', false);
                    }
                  }}
                  size="lg"
                />
              </FormControl>

              <Divider borderColor="blue.100" />

              <FormControl display="flex" alignItems="center">
                <FormLabel htmlFor="is_assembly" mb="0" fontWeight="bold" flex="1">
                  Produk Hasil Rakitan (Target Assembly)
                  <Text fontSize="sm" fontWeight="normal" color="gray.600">
                    Aktifkan jika produk dibuat dari berbagai komponen (BOM). Biaya pokok akan dihitung otomatis.
                  </Text>
                </FormLabel>
                <Switch
                  id="is_assembly"
                  isChecked={formData.is_assembly}
                  onChange={(e) => handleInputChange('is_assembly', e.target.checked)}
                  size="lg"
                  colorScheme="teal"
                />
              </FormControl>

              <Divider borderColor="blue.200" />

              <HStack spacing={8}>
                <FormControl display="flex" alignItems="center" w="auto">
                  <FormLabel htmlFor="is_active" mb="0" mr={3}>
                    {t('products.form.isActive')}
                  </FormLabel>
                  <Switch
                    id="is_active"
                    isChecked={formData.is_active}
                    onChange={(e) => handleInputChange('is_active', e.target.checked)}
                  />
                </FormControl>

                <FormControl display="flex" alignItems="center" w="auto">
                  <FormLabel htmlFor="taxable" mb="0" mr={3}>
                    {t('products.form.taxable')}
                  </FormLabel>
                  <Switch
                    id="taxable"
                    isChecked={formData.taxable}
                    onChange={(e) => handleInputChange('taxable', e.target.checked)}
                  />
                </FormControl>
              </HStack>
            </VStack>
          </Box>

          <Grid templateColumns="repeat(4, 1fr)" gap={4}>
            <GridItem colSpan={2}>
              <FormControl isRequired>
                <FormLabel>{t('products.productCode')}</FormLabel>
                <Input
                  value={formData.code}
                  onChange={(e) => handleInputChange('code', e.target.value)}
                  placeholder={t('products.form.enterProductCode')}
                />
              </FormControl>
            </GridItem>
            <GridItem colSpan={2}>
              <FormControl isRequired>
                <FormLabel>{t('products.productName')}</FormLabel>
                <Input
                  value={formData.name}
                  onChange={(e) => handleInputChange('name', e.target.value)}
                  placeholder={t('products.form.enterProductName')}
                />
              </FormControl>
            </GridItem>
            <GridItem colSpan={4}>
              <FormControl>
                <FormLabel>{t('products.description')}</FormLabel>
                <Textarea
                  value={formData.description}
                  onChange={(e) => handleInputChange('description', e.target.value)}
                  placeholder={t('products.form.enterDescription')}
                />
              </FormControl>
            </GridItem>
            <GridItem>
              <FormControl>
                <FormLabel>{t('products.categoryLabel')}</FormLabel>
                <Select
                  value={formData.category_id || ''}
                  onChange={(e) => handleInputChange('category_id', e.target.value ? Number(e.target.value) : undefined)}
                  placeholder={t('products.form.selectCategory')}
                >
                  {categories.map(category => (
                    <option key={category.id} value={category.id}>
                      {category.name}
                    </option>
                  ))}
                </Select>
              </FormControl>
            </GridItem>

            {/* Warehouse Location only for Non-Service */}
            {!formData.is_service && (
              <GridItem>
                <FormControl>
                  <FormLabel>{t('products.form.warehouseLocation')}</FormLabel>
                  <Select
                    value={formData.warehouse_location_id || ''}
                    onChange={(e) => handleInputChange('warehouse_location_id', e.target.value ? Number(e.target.value) : undefined)}
                    placeholder={t('products.form.selectWarehouse')}
                  >
                    {warehouseLocations.map(location => (
                      <option key={location.id} value={location.id}>
                        {location.name} ({location.code})
                      </option>
                    ))}
                  </Select>
                </FormControl>
              </GridItem>
            )}

            <GridItem colSpan={formData.is_service ? 3 : 2}>
              <FormControl isRequired>
                <FormLabel>{t('products.unit')}</FormLabel>
                <Select
                  value={formData.unit}
                  onChange={(e) => handleInputChange('unit', e.target.value)}
                  placeholder={t('products.form.selectUnit')}
                >
                  {units.map(unit => (
                    <option key={unit.code} value={unit.code}>
                      {unit.name} ({unit.code})
                    </option>
                  ))}
                </Select>
              </FormControl>
            </GridItem>
          </Grid>
        </Box>

        <Divider />

        {/* Product Details - Only for Non-Service */}
        {!formData.is_service && (
          <>
            <Box>
              <Text fontSize="lg" fontWeight="bold" mb={4}>{t('products.form.productDetails')}</Text>
              <Grid templateColumns="repeat(4, 1fr)" gap={4}>
                <GridItem colSpan={2}>
                  <FormControl>
                    <FormLabel>{t('products.form.brand')}</FormLabel>
                    <Input
                      value={formData.brand}
                      onChange={(e) => handleInputChange('brand', e.target.value)}
                      placeholder={t('products.form.enterBrand')}
                    />
                  </FormControl>
                </GridItem>
                <GridItem colSpan={2}>
                  <FormControl>
                    <FormLabel>{t('products.form.model')}</FormLabel>
                    <Input
                      value={formData.model}
                      onChange={(e) => handleInputChange('model', e.target.value)}
                      placeholder={t('products.form.enterModel')}
                    />
                  </FormControl>
                </GridItem>
                <GridItem colSpan={2}>
                  <FormControl>
                    <FormLabel>{t('products.form.barcode')}</FormLabel>
                    <Input
                      value={formData.barcode}
                      onChange={(e) => handleInputChange('barcode', e.target.value)}
                      placeholder={t('products.form.enterBarcode')}
                    />
                  </FormControl>
                </GridItem>
                <GridItem colSpan={2}>
                  <FormControl>
                    <FormLabel>{t('products.form.sku')}</FormLabel>
                    <Input
                      value={formData.sku}
                      onChange={(e) => handleInputChange('sku', e.target.value)}
                      placeholder={t('products.form.enterSKU')}
                    />
                  </FormControl>
                </GridItem>
                <GridItem>
                  <FormControl>
                    <FormLabel>{t('products.form.weight')}</FormLabel>
                    <NumberInput
                      value={formData.weight || 0}
                      onChange={(_, value) => handleInputChange('weight', isNaN(value) ? 0 : value)}
                      min={0}
                      precision={3}
                    >
                      <NumberInputField />
                      <NumberInputStepper>
                        <NumberIncrementStepper />
                        <NumberDecrementStepper />
                      </NumberInputStepper>
                    </NumberInput>
                  </FormControl>
                </GridItem>
                <GridItem>
                  <FormControl>
                    <FormLabel>{t('products.form.dimensions')}</FormLabel>
                    <Input
                      value={formData.dimensions}
                      onChange={(e) => handleInputChange('dimensions', e.target.value)}
                      placeholder="P x L x T (cm)"
                    />
                  </FormControl>
                </GridItem>
              </Grid>
            </Box>

            <Divider />
          </>
        )}

        {/* Pricing */}
        <Box>
          <Text fontSize="lg" fontWeight="bold" mb={4}>{t('products.form.pricing')}</Text>
          <Grid templateColumns={formData.is_service ? "repeat(1, 1fr)" : "repeat(3, 1fr)"} gap={4}>
            {/* Harga Beli - Only for Non-Service */}
            {!formData.is_service && (
              <GridItem>
                <CurrencyInput
                  value={formData.purchase_price || 0}
                  onChange={(value) => handleInputChange('purchase_price', value)}
                  label={t('products.purchasePrice')}
                  placeholder="Contoh: Rp 50.000"
                  isRequired={!formData.is_assembly}
                  isDisabled={formData.is_assembly}
                  size="md"
                  min={0}
                />
              </GridItem>
            )}
            {/* Harga Pokok (COGS) - Only for Non-Service */}
            {!formData.is_service && (
              <GridItem>
                <CurrencyInput
                  value={formData.cost_price || 0}
                  onChange={(value) => handleInputChange('cost_price', value)}
                  label={t('products.costPrice')}
                  placeholder="Contoh: Rp 300.000"
                  isRequired={!formData.is_assembly}
                  isDisabled={formData.is_assembly}
                  size="md"
                  min={0}
                />
              </GridItem>
            )}
            {/* Harga Jual - Always shown */}
            <GridItem>
              <CurrencyInput
                value={formData.sale_price || 0}
                onChange={(value) => handleInputChange('sale_price', value)}
                label={t('products.salePrice')}
                placeholder="Contoh: Rp 1.200.000"
                isRequired={!formData.is_assembly}
                size="md"
                min={0}
              />
            </GridItem>
            <GridItem colSpan={formData.is_service ? 1 : 3}>
              <FormControl>
                <FormLabel>{t('products.pricingTier')}</FormLabel>
                <Select
                  value={formData.pricing_tier}
                  onChange={(e) => handleInputChange('pricing_tier', e.target.value)}
                  placeholder={t('products.form.selectPricingTier')}
                >
                  {pricingTiers.map(tier => (
                    <option key={tier} value={tier}>{tier}</option>
                  ))}
                </Select>
              </FormControl>
            </GridItem>
          </Grid>
        </Box>

        <Divider />

        {/* Inventory - Only for Non-Service */}
        {!formData.is_service && (
          <>
            <Box>
              <Text fontSize="lg" fontWeight="bold" mb={4}>{t('products.form.inventory')}</Text>
              <Grid templateColumns="repeat(4, 1fr)" gap={4}>
                <GridItem>
                  <FormControl>
                    <FormLabel>{t('products.currentStock')}</FormLabel>
                    <NumberInput
                      value={formData.stock || 0}
                      onChange={(_, value) => handleInputChange('stock', isNaN(value) ? 0 : value)}
                      min={0}
                    >
                      <NumberInputField />
                      <NumberInputStepper>
                        <NumberIncrementStepper />
                        <NumberDecrementStepper />
                      </NumberInputStepper>
                    </NumberInput>
                  </FormControl>
                </GridItem>
                <GridItem>
                  <FormControl>
                    <FormLabel>{t('products.minStock')}</FormLabel>
                    <NumberInput
                      value={formData.min_stock || 0}
                      onChange={(_, value) => handleInputChange('min_stock', isNaN(value) ? 0 : value)}
                      min={0}
                    >
                      <NumberInputField />
                      <NumberInputStepper>
                        <NumberIncrementStepper />
                        <NumberDecrementStepper />
                      </NumberInputStepper>
                    </NumberInput>
                  </FormControl>
                </GridItem>
                <GridItem>
                  <FormControl>
                    <FormLabel>{t('products.maxStock')}</FormLabel>
                    <NumberInput
                      value={formData.max_stock || 0}
                      onChange={(_, value) => handleInputChange('max_stock', isNaN(value) ? 0 : value)}
                      min={0}
                    >
                      <NumberInputField />
                      <NumberInputStepper>
                        <NumberIncrementStepper />
                        <NumberDecrementStepper />
                      </NumberInputStepper>
                    </NumberInput>
                  </FormControl>
                </GridItem>
                <GridItem>
                  <FormControl>
                    <FormLabel>{t('products.reorderLevel')}</FormLabel>
                    <NumberInput
                      value={formData.reorder_level || 0}
                      onChange={(_, value) => handleInputChange('reorder_level', isNaN(value) ? 0 : value)}
                      min={0}
                    >
                      <NumberInputField />
                      <NumberInputStepper>
                        <NumberIncrementStepper />
                        <NumberDecrementStepper />
                      </NumberInputStepper>
                    </NumberInput>
                  </FormControl>
                </GridItem>
              </Grid>
            </Box>

            <Divider />
          </>
        )}

        {/* Product Image */}
        <Box>
          <Text fontSize="lg" fontWeight="bold" mb={4}>{t('products.form.productImage')}</Text>
          <Grid templateColumns="repeat(3, 1fr)" gap={4}>
            {/* Current Image */}
            <GridItem>
              <FormControl>
                <FormLabel>{t('products.form.currentImage')}</FormLabel>
                {formData.image_path ? (
                  <Image
                    src={getProductImageUrl(formData.image_path) || ''}
                    alt={formData.name || 'Product image'}
                    maxH="150px"
                    maxW="200px"
                    objectFit="contain"
                    borderRadius="md"
                    border="1px"
                    borderColor="gray.200"
                    fallbackSrc="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='200' height='150' viewBox='0 0 200 150'%3E%3Crect width='200' height='150' fill='%23f0f0f0'/%3E%3Ctext x='100' y='75' text-anchor='middle' dy='.3em' font-family='Arial, sans-serif' font-size='12' fill='%23999'%3EImage not found%3C/text%3E%3C/svg%3E"
                    onError={(e) => {
                      console.error('Image failed to load:', formData.image_path);
                      console.error('Attempted URL:', getProductImageUrl(formData.image_path));
                      debugImageUrl(formData.image_path);
                    }}
                  />
                ) : (
                  <Box
                    w="200px"
                    h="150px"
                    bg="gray.100"
                    borderRadius="md"
                    display="flex"
                    alignItems="center"
                    justifyContent="center"
                  >
                    <Text color="gray.500">{t('products.form.noImage')}</Text>
                  </Box>
                )}
              </FormControl>
            </GridItem>

            {/* Image Upload */}
            <GridItem>
              <FormControl>
                <FormLabel>{t('products.form.uploadNewImage')}</FormLabel>
                <Input
                  type="file"
                  accept="image/*"
                  onChange={handleImageChange}
                  mb={2}
                />
                {imagePreview && (
                  <Image
                    src={imagePreview}
                    alt="Preview"
                    maxH="150px"
                    maxW="200px"
                    objectFit="contain"
                    borderRadius="md"
                    border="1px"
                    borderColor="gray.200"
                  />
                )}
              </FormControl>
            </GridItem>

            {/* Upload Button */}
            <GridItem>
              <FormControl>
                <FormLabel>{t('common.actions')}</FormLabel>
                <Button
                  onClick={handleImageUpload}
                  colorScheme="blue"
                  isDisabled={!selectedImage || !formData.id}
                  size="sm"
                >
                  {t('products.form.uploadImage')}
                </Button>
                {!formData.id && (
                  <Text fontSize="xs" color="gray.500" mt={2}>
                    {t('products.form.saveProductFirst')}
                  </Text>
                )}
              </FormControl>
            </GridItem>
          </Grid>
        </Box>

        <Divider />

        {/* Settings */}
        <Box>
          <VStack spacing={4} align="stretch">
            {/* Status switches moved to top */}
            <FormControl>
              <FormLabel>{t('common.notes')}</FormLabel>
              <Textarea
                value={formData.notes}
                onChange={(e) => handleInputChange('notes', e.target.value)}
                placeholder={t('products.form.additionalNotes')}
              />
            </FormControl>
          </VStack>
        </Box>

        {/* Form Actions */}
        <HStack justify="flex-end" spacing={4}>
          <Button
            leftIcon={<FiX />}
            onClick={onCancel}
            variant="outline"
          >
            {t('common.cancel')}
          </Button>
          <Button
            leftIcon={<FiSave />}
            type="submit"
            colorScheme="blue"
            isLoading={isLoading}
            loadingText={product?.id ? t('common.updating') : t('common.creating')}
          >
            {product?.id ? t('products.updateProduct') : t('products.createProduct')}
          </Button>
        </HStack>
      </VStack>
    </Box>
  );
};

export default ProductForm;

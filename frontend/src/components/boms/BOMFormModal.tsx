import React, { useState, useEffect } from 'react';
import {
    Modal,
    ModalOverlay,
    ModalContent,
    ModalHeader,
    ModalCloseButton,
    ModalBody,
    ModalFooter,
    Button,
    FormControl,
    FormLabel,
    Input,
    VStack,
    HStack,
    Flex,
    FormErrorMessage,
    useToast,
    Textarea,
    Box,
    Heading,
    Table,
    Thead,
    Tbody,
    Tr,
    Th,
    Td,
    IconButton,
    NumberInput,
    NumberInputField,
    Text,
    Divider,
} from '@chakra-ui/react';
import { FiPlus, FiTrash2 } from 'react-icons/fi';
import { useForm, Controller, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import * as z from 'zod';

import { bomService } from '@/services/bomService';
import ProductService, { Product } from '@/services/productService';
import { BOM, BOMItemCreatePayload } from '@/types/bom';
import SearchableSelect from '@/components/common/SearchableSelect';
import CurrencyInput from '@/components/common/CurrencyInput';

const bomItemSchema = z.object({
    component_product_id: z.number().min(1, 'Product is required'),
    category_str: z.string().min(1, 'Category is required'),
    base_quantity: z.number().min(0.01, 'Quantity must be > 0'),
    base_unit: z.string().min(1, 'Unit is required'),
    notes: z.string().optional(),
    unit_price: z.number().min(0, 'Price must be >= 0'),
    requirement_quantity: z.number().min(0.01, 'Req qty must be > 0'),
});

const formSchema = z.object({
    code: z.string().min(1, 'Code is required'),
    name: z.string().min(1, 'Name is required'),
    description: z.string().optional(),
    target_product_id: z.number().min(1, 'Target product is required'),
    target_quantity: z.number().min(1, 'Quantity must be at least 1'),
    items: z.array(bomItemSchema).min(1, 'At least one component is required'),
});

type FormData = z.infer<typeof formSchema>;

interface BOMFormModalProps {
    isOpen: boolean;
    onClose: () => void;
    onSuccess: () => void;
    existingBOM?: BOM | null;
}

const BOMFormModal: React.FC<BOMFormModalProps> = ({
    isOpen,
    onClose,
    onSuccess,
    existingBOM,
}) => {
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [products, setProducts] = useState<Product[]>([]);
    const [productOptions, setProductOptions] = useState<{ id: string | number; name: string }[]>([]);
    const toast = useToast();

    const {
        control,
        handleSubmit,
        register,
        reset,
        watch,
        setValue,
        formState: { errors },
    } = useForm<FormData>({
        resolver: zodResolver(formSchema),
        defaultValues: {
            code: '',
            name: '',
            description: '',
            target_product_id: 0,
            target_quantity: 1,
            items: [],
        },
    });

    const { fields, append, remove } = useFieldArray({
        control,
        name: 'items',
    });

    const watchedItems = watch('items');

    useEffect(() => {
        fetchProducts();
    }, []);

    useEffect(() => {
        if (existingBOM && isOpen) {
            reset({
                code: existingBOM.code,
                name: existingBOM.name,
                description: existingBOM.description,
                target_product_id: existingBOM.target_product_id,
                target_quantity: existingBOM.target_quantity,
                items: existingBOM.items.map((item) => ({
                    component_product_id: item.component_product_id,
                    category_str: item.category_str,
                    base_quantity: item.base_quantity,
                    base_unit: item.base_unit,
                    notes: item.notes,
                    unit_price: item.unit_price,
                    requirement_quantity: item.requirement_quantity,
                })),
            });
        } else if (isOpen) {
            reset({
                code: '',
                name: '',
                description: '',
                target_product_id: 0,
                target_quantity: 1,
                items: [],
            });
        }
    }, [existingBOM, isOpen, reset]);

    const fetchProducts = async () => {
        try {
            const response = await ProductService.getProducts({ limit: 1000 });
            if (response.data) {
                setProducts(response.data);
                setProductOptions(
                    response.data.map((p: Product) => ({ id: p.id, name: `${p.code} - ${p.name}` }))
                );
            }
        } catch (error) {
            console.error('Failed to fetch products', error);
        }
    };

    const handleProductSelect = (index: number, productIdStr: string | number) => {
        const pId = typeof productIdStr === 'string' ? parseInt(productIdStr) : productIdStr;
        const product = products.find((p) => p.id === pId);
        if (product) {
            setValue(`items.${index}.component_product_id`, pId);
            setValue(`items.${index}.unit_price`, product.purchase_price || 0);
            setValue(`items.${index}.base_unit`, product.unit || '');
        }
    };

    const onSubmit = async (data: FormData) => {
        setIsSubmitting(true);
        const payload = {
            ...data,
            description: data.description || '',
        };
        try {
            if (existingBOM) {
                // bomService.updateBOM not implemented yet, using create for now representing overwrite
                // In a real scenario, implement standard PUT request for existing entries
                toast({
                    title: 'Update function not completely wired, creating new logic placeholder',
                    status: 'info',
                });
            } else {
                await bomService.createBOM(payload);
                toast({
                    title: 'BOM created successfully',
                    status: 'success',
                    isClosable: true,
                });
            }
            onSuccess();
            onClose();
        } catch (error: any) {
            toast({
                title: 'Failed to save BOM',
                description: error.message,
                status: 'error',
                isClosable: true,
            });
        } finally {
            setIsSubmitting(false);
        }
    };

    const calculateTotal = () => {
        return watchedItems.reduce((sum, item) => {
            return sum + (item.requirement_quantity || 0) * (item.unit_price || 0);
        }, 0);
    };

    return (
        <Modal isOpen={isOpen} onClose={onClose} size="6xl" scrollBehavior="inside">
            <ModalOverlay />
            <ModalContent my={4} maxH="90vh">
                <form onSubmit={handleSubmit(onSubmit)}>
                    <ModalHeader>
                        {existingBOM ? 'Edit BOM Template' : 'Create New BOM Template'}
                    </ModalHeader>
                    <ModalCloseButton />

                    <ModalBody pb={6}>
                        <VStack spacing={6} align="stretch">
                            <Box p={4} borderWidth="1px" borderRadius="md" bg="gray.50">
                                <Heading size="sm" mb={4}>Header Information</Heading>
                                <HStack spacing={4} align="flex-start" mb={4}>
                                    <FormControl isInvalid={!!errors.code} isRequired>
                                        <FormLabel>BOM Code</FormLabel>
                                        <Input {...register('code')} placeholder="e.g. BOM-001" bg="white" />
                                        <FormErrorMessage>{errors.code?.message}</FormErrorMessage>
                                    </FormControl>

                                    <FormControl isInvalid={!!errors.name} isRequired>
                                        <FormLabel>BOM Name</FormLabel>
                                        <Input {...register('name')} placeholder="e.g. Lapangan Padel Standard" bg="white" />
                                        <FormErrorMessage>{errors.name?.message}</FormErrorMessage>
                                    </FormControl>
                                </HStack>

                                <HStack spacing={4} align="flex-start" mb={4}>
                                    <FormControl isInvalid={!!errors.target_product_id} isRequired flex={2}>
                                        <FormLabel>Target Assembly Product</FormLabel>
                                        <Controller
                                            control={control}
                                            name="target_product_id"
                                            render={({ field }) => (
                                                <SearchableSelect
                                                    options={productOptions}
                                                    value={field.value ? field.value.toString() : ''}
                                                    onChange={(val) => field.onChange(typeof val === 'string' ? parseInt(val) : val)}
                                                    placeholder="Search product..."
                                                />
                                            )}
                                        />
                                        <FormErrorMessage>{errors.target_product_id?.message}</FormErrorMessage>
                                    </FormControl>

                                    <FormControl isInvalid={!!errors.target_quantity} isRequired flex={1}>
                                        <FormLabel>Target Qty</FormLabel>
                                        <Controller
                                            control={control}
                                            name="target_quantity"
                                            render={({ field }) => (
                                                <NumberInput
                                                    min={1}
                                                    value={field.value}
                                                    onChange={(_, val) => field.onChange(val)}
                                                    bg="white"
                                                >
                                                    <NumberInputField />
                                                </NumberInput>
                                            )}
                                        />
                                        <FormErrorMessage>{errors.target_quantity?.message}</FormErrorMessage>
                                    </FormControl>
                                </HStack>

                                <FormControl isInvalid={!!errors.description}>
                                    <FormLabel>Description / Notes</FormLabel>
                                    <Textarea {...register('description')} rows={2} bg="white" />
                                </FormControl>
                            </Box>

                            <Divider />

                            <Box>
                                <Flex justify="space-between" align="center" mb={4}>
                                    <Heading size="sm">Components (Base Cost Calculator)</Heading>
                                    <Button
                                        leftIcon={<FiPlus />}
                                        size="sm"
                                        colorScheme="blue"
                                        variant="outline"
                                        onClick={() => append({
                                            component_product_id: 0,
                                            category_str: '',
                                            base_quantity: 1,
                                            base_unit: '',
                                            notes: '',
                                            unit_price: 0,
                                            requirement_quantity: 1,
                                        })}
                                    >
                                        Add Component
                                    </Button>
                                </Flex>

                                <Box overflowX="auto" borderWidth="1px" borderRadius="md">
                                    <Table size="sm" variant="simple">
                                        <Thead bg="gray.100">
                                            <Tr>
                                                <Th width="200px">Material</Th>
                                                <Th width="150px">Category (Group)</Th>
                                                <Th width="100px" isNumeric>Qty (Persiapan)</Th>
                                                <Th width="100px">Unit (Persiapan)</Th>
                                                <Th>Notes</Th>
                                                <Th width="120px" isNumeric>Required Qty</Th>
                                                <Th width="150px" isNumeric>Unit Price</Th>
                                                <Th width="150px" isNumeric>Total Cost</Th>
                                                <Th width="50px"></Th>
                                            </Tr>
                                        </Thead>
                                        <Tbody>
                                            {fields.map((field, index) => {
                                                const currentUnitPrice = watchedItems[index]?.unit_price || 0;
                                                const currentReqQty = watchedItems[index]?.requirement_quantity || 0;
                                                const lineTotal = currentUnitPrice * currentReqQty;

                                                return (
                                                    <Tr key={field.id}>
                                                        <Td>
                                                            <Controller
                                                                control={control}
                                                                name={`items.${index}.component_product_id`}
                                                                render={({ field: selectField }) => (
                                                                    <SearchableSelect
                                                                        options={productOptions}
                                                                        value={selectField.value ? selectField.value.toString() : ''}
                                                                        onChange={(val) => {
                                                                            selectField.onChange(typeof val === 'string' ? parseInt(val) : val);
                                                                            handleProductSelect(index, val);
                                                                        }}
                                                                        placeholder="Select material..."
                                                                        size="sm"
                                                                    />
                                                                )}
                                                            />
                                                        </Td>
                                                        <Td>
                                                            <Input size="sm" {...register(`items.${index}.category_str`)} placeholder="e.g. A. Kelengkapan" />
                                                        </Td>
                                                        <Td isNumeric>
                                                            <Controller
                                                                control={control}
                                                                name={`items.${index}.base_quantity`}
                                                                render={({ field: numField }) => (
                                                                    <NumberInput
                                                                        size="sm"
                                                                        min={0.01}
                                                                        value={numField.value}
                                                                        onChange={(_, val) => numField.onChange(val)}
                                                                    >
                                                                        <NumberInputField />
                                                                    </NumberInput>
                                                                )}
                                                            />
                                                        </Td>
                                                        <Td>
                                                            <Input size="sm" {...register(`items.${index}.base_unit`)} placeholder="e.g. L, Pcs" />
                                                        </Td>
                                                        <Td>
                                                            <Input size="sm" {...register(`items.${index}.notes`)} placeholder="Keterangan..." />
                                                        </Td>
                                                        <Td isNumeric>
                                                            <Controller
                                                                control={control}
                                                                name={`items.${index}.requirement_quantity`}
                                                                render={({ field: numField }) => (
                                                                    <NumberInput
                                                                        size="sm"
                                                                        min={0.01}
                                                                        value={numField.value}
                                                                        onChange={(_, val) => numField.onChange(val)}
                                                                    >
                                                                        <NumberInputField />
                                                                    </NumberInput>
                                                                )}
                                                            />
                                                        </Td>
                                                        <Td isNumeric>
                                                            <Controller
                                                                control={control}
                                                                name={`items.${index}.unit_price`}
                                                                render={({ field: currencyField }) => (
                                                                    <CurrencyInput
                                                                        value={currencyField.value}
                                                                        onChange={(val) => currencyField.onChange(val)}
                                                                        placeholder="0"
                                                                        size="sm"
                                                                    />
                                                                )}
                                                            />
                                                        </Td>
                                                        <Td isNumeric fontWeight="bold" color="blue.600">
                                                            {new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(lineTotal)}
                                                        </Td>
                                                        <Td>
                                                            <IconButton
                                                                aria-label="Remove item"
                                                                icon={<FiTrash2 />}
                                                                size="sm"
                                                                colorScheme="red"
                                                                variant="ghost"
                                                                onClick={() => remove(index)}
                                                            />
                                                        </Td>
                                                    </Tr>
                                                );
                                            })}
                                            {fields.length === 0 && (
                                                <Tr>
                                                    <Td colSpan={9} textAlign="center" py={4} color="gray.500">
                                                        No components added. Click "Add Component" to start building the BOM.
                                                    </Td>
                                                </Tr>
                                            )}
                                            {!!errors.items && (
                                                <Tr>
                                                    <Td colSpan={9} textAlign="center" color="red.500" py={2} bg="red.50">
                                                        {errors.items.message as string}
                                                    </Td>
                                                </Tr>
                                            )}
                                        </Tbody>
                                    </Table>
                                </Box>

                                <Flex justify="flex-end" mt={4} p={4} bg="gray.100" borderRadius="md">
                                    <Heading size="md">
                                        Total Unit Cost: <Text as="span" color="blue.600">{new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(calculateTotal())}</Text>
                                    </Heading>
                                </Flex>

                            </Box>
                        </VStack>
                    </ModalBody>

                    <ModalFooter borderTopWidth="1px" bg="gray.50">
                        <Button variant="ghost" mr={3} onClick={onClose} isDisabled={isSubmitting}>
                            Cancel
                        </Button>
                        <Button
                            colorScheme="blue"
                            type="submit"
                            isLoading={isSubmitting}
                        >
                            Save BOM Template
                        </Button>
                    </ModalFooter>
                </form>
            </ModalContent>
        </Modal>
    );
};

export default BOMFormModal;

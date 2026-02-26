import React, { useState, useRef } from 'react';
import {
    Modal, ModalOverlay, ModalContent, ModalHeader, ModalFooter, ModalBody, ModalCloseButton,
    Button, VStack, HStack, Text, Select, Table, Thead, Tbody, Tr, Th, Td, Box, useToast, Icon,
    FormControl, FormLabel
} from '@chakra-ui/react';
import { FiUploadCloud, FiCheckCircle } from 'react-icons/fi';
import productService, { Category, Product } from '@/services/productService';
import { parseCSV, cleanCurrencyString } from '@/utils/csvParser';

interface ProductImportModalProps {
    isOpen: boolean;
    onClose: () => void;
    onSuccess: () => void;
    categories: Category[];
}

const STEPS = ['Upload File', 'Map Columns', 'Review & Import'];

export default function ProductImportModal({ isOpen, onClose, onSuccess, categories }: ProductImportModalProps) {
    const [activeStep, setActiveStep] = useState(0);
    const [file, setFile] = useState<File | null>(null);
    const [csvData, setCsvData] = useState<string[][]>([]);
    const [headerRowIndex, setHeaderRowIndex] = useState(0);
    const [isUploading, setIsUploading] = useState(false);
    const toast = useToast();
    const fileInputRef = useRef<HTMLInputElement>(null);

    // Mapping state: destinationField -> csvColumnIndex
    const [mapping, setMapping] = useState<{ [key: string]: string }>({
        name: '',
        code: '',
        category_id: '',
        purchase_price: '',
        sale_price: '',
        unit: '',
        description: '',
    });

    // Auto-map columns whenever headerRowIndex or csvData changes
    React.useEffect(() => {
        if (!csvData.length || headerRowIndex < 0 || headerRowIndex >= csvData.length) return;

        const headers = csvData[headerRowIndex];
        const autoMapping: { [key: string]: string } = {
            name: '', code: '', category_id: '', purchase_price: '', sale_price: '', unit: '', description: ''
        };

        headers.forEach((header, idx) => {
            if (!header) return;
            const h = header.toLowerCase().trim();

            // Name / Description
            if (h === 'deskripsi' || h.includes('nama') || h.includes('name') || h.includes('item') || h.includes('barang') || h.includes('produk')) {
                if (!autoMapping.name) autoMapping.name = idx.toString();
                else if (!autoMapping.description && h.includes('deskripsi')) autoMapping.description = idx.toString();
            }

            // Description (explicit)
            if (h.includes('keterangan') || h.includes('catatan') || h === 'desc' || h === 'description') {
                if (!autoMapping.description && autoMapping.name !== idx.toString()) autoMapping.description = idx.toString();
            }

            // Code
            if (h.includes('kode') || h.includes('code') || h === 'no' || h === 'no.' || h.includes('nomor') || h.includes('sku')) {
                if (!autoMapping.code) autoMapping.code = idx.toString();
            }

            // Category
            if (h.includes('kategori') || h.includes('category') || h.includes('golongan') || h.includes('jenis')) {
                if (!autoMapping.category_id) autoMapping.category_id = idx.toString();
            }

            // Purchase Price / Cost
            if (h.includes('beli') || h.includes('pokok') || h.includes('cost') || h.includes('purchase') || h.includes('hpp') || h.includes('modal') || h.includes('harga per pcs')) {
                if (!autoMapping.purchase_price) autoMapping.purchase_price = idx.toString();
            }

            // Sale Price (Jual / Total Harga / Retail)
            if (h.includes('jual') || h.includes('sale') || h.includes('retail') || h.includes('total harg')) {
                if (!autoMapping.sale_price) autoMapping.sale_price = idx.toString();
            }

            // Generic "Harga" or "Price" fallback
            if (h.includes('harga') || h.includes('price')) {
                // If it wasn't caught by Beli/Cost or Jual/Sale
                if (!h.includes('beli') && !h.includes('pokok') && !h.includes('cost') && !h.includes('purchase') && !h.includes('jual') && !h.includes('sale') && !h.includes('total')) {
                    if (!autoMapping.sale_price) {
                        autoMapping.sale_price = idx.toString();
                    } else if (!autoMapping.purchase_price) {
                        autoMapping.purchase_price = idx.toString();
                    }
                }
            }

            // Unit
            if (h.includes('satuan') || h.includes('unit') || h === 'sat' || h === 'uom') {
                if (!autoMapping.unit) autoMapping.unit = idx.toString();
            }
        });

        setMapping(autoMapping);
    }, [csvData, headerRowIndex]);

    const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        if (e.target.files && e.target.files.length > 0) {
            const selectedFile = e.target.files[0];
            setFile(selectedFile);

            const reader = new FileReader();
            reader.onload = (event) => {
                const text = event.target?.result as string;
                const parsed = parseCSV(text);
                setCsvData(parsed);
                // auto-detect header row (first row with most columns)
                let maxCols = 0;
                let bestRow = 0;
                parsed.slice(0, 15).forEach((row, idx) => {
                    const cols = row.filter(c => c.trim() !== '').length;
                    if (cols > maxCols) {
                        maxCols = cols;
                        bestRow = idx;
                    }
                });
                setHeaderRowIndex(bestRow);
                setActiveStep(1);
            };
            reader.readAsText(selectedFile);
        }
    };

    const handleMappingChange = (field: string, csvIndex: string) => {
        setMapping(prev => ({ ...prev, [field]: csvIndex }));
    };

    const processImport = async () => {
        if (!mapping.name) {
            toast({ title: 'Name mapping is required', status: 'error' });
            return;
        }

        setIsUploading(true);
        try {
            const productsToImport: Partial<Product>[] = [];

            for (let i = headerRowIndex + 1; i < csvData.length; i++) {
                const row = csvData[i];
                if (row.every(cell => !cell?.trim())) continue; // skip empty rows

                const getVal = (idxStr: string) => {
                    if (!idxStr) return '';
                    const idx = parseInt(idxStr);
                    return row[idx] || '';
                };

                const name = getVal(mapping.name);
                if (!name) continue; // skip rows without name

                // Try to match category by name if a name was provided instead of ID
                let categoryId: number | undefined = undefined;
                let catText = getVal(mapping.category_id);
                if (catText) {
                    const idMatch = parseInt(catText);
                    if (!isNaN(idMatch) && idMatch > 0) {
                        categoryId = idMatch;
                    } else {
                        // Try finding category by name
                        const foundCat = categories.find(c => c.name.toLowerCase() === catText.trim().toLowerCase());
                        if (foundCat) categoryId = foundCat.id;
                    }
                }

                const product: Partial<Product> = {
                    name,
                    code: getVal(mapping.code),
                    unit: getVal(mapping.unit) || 'Pcs',
                    description: getVal(mapping.description),
                    purchase_price: cleanCurrencyString(getVal(mapping.purchase_price)),
                    cost_price: cleanCurrencyString(getVal(mapping.purchase_price)),
                    sale_price: cleanCurrencyString(getVal(mapping.sale_price)),
                    category_id: categoryId,
                    is_active: true,
                    stock: 0,
                };
                productsToImport.push(product);
            }

            if (productsToImport.length === 0) {
                toast({ title: 'No valid products found to import', status: 'warning' });
                setIsUploading(false);
                return;
            }

            const res = await productService.importProducts(productsToImport);
            toast({
                title: 'Import successful',
                description: `${res.imported || 0} imported, ${res.updated || 0} updated.`,
                status: 'success'
            });
            onSuccess();
            onClose();
        } catch (error: any) {
            toast({
                title: 'Import failed',
                description: error.response?.data?.error || error.message,
                status: 'error',
            });
        } finally {
            setIsUploading(false);
        }
    };

    const renderStepContent = () => {
        switch (activeStep) {
            case 0:
                return (
                    <VStack spacing={4} py={8}>
                        <Icon as={FiUploadCloud} boxSize={12} color="brand.500" />
                        <Text textAlign="center">Upload a CSV or Excel file (saved as CSV) containing your products or standard costs.</Text>
                        <input type="file" accept=".csv" ref={fileInputRef} hidden onChange={handleFileChange} />
                        <Button colorScheme="brand" onClick={() => fileInputRef.current?.click()}>
                            Select File
                        </Button>
                    </VStack>
                );
            case 1:
                if (!csvData.length) return null;
                const headers = csvData[headerRowIndex] || [];
                return (
                    <VStack spacing={4} align="stretch">
                        <FormControl>
                            <FormLabel>Select Header Row</FormLabel>
                            <Select value={headerRowIndex} onChange={(e) => setHeaderRowIndex(parseInt(e.target.value))}>
                                {csvData.slice(0, 15).map((row, idx) => (
                                    <option key={idx} value={idx}>
                                        Row {idx + 1}: {row.filter(c => c).join(', ').substring(0, 80)}...
                                    </option>
                                ))}
                            </Select>
                        </FormControl>

                        <Box overflowX="auto" overflowY="auto" maxHeight="350px" borderWidth="1px" borderRadius="md">
                            <Table size="sm" variant="simple">
                                <Thead bg="gray.50" position="sticky" top={0} zIndex={1}>
                                    <Tr>
                                        <Th>Product Field</Th>
                                        <Th>CSV Column Mapping</Th>
                                    </Tr>
                                </Thead>
                                <Tbody>
                                    {[
                                        { key: 'name', label: 'Nama Barang *', required: true },
                                        { key: 'code', label: 'Kode Barang' },
                                        { key: 'category_id', label: 'Kategori (Nama / ID)' },
                                        { key: 'purchase_price', label: 'Acuan Harga Beli / Harga Pokok' },
                                        { key: 'sale_price', label: 'Acuan Harga Jual' },
                                        { key: 'unit', label: 'Satuan' },
                                        { key: 'description', label: 'Deskripsi Tambahan' },
                                    ].map(field => (
                                        <Tr key={field.key}>
                                            <Td fontWeight={field.required ? 'bold' : 'normal'}>{field.label}</Td>
                                            <Td>
                                                <Select
                                                    placeholder="-- Ignore --"
                                                    size="sm"
                                                    value={mapping[field.key]}
                                                    onChange={(e) => handleMappingChange(field.key, e.target.value)}
                                                >
                                                    {headers.map((h, idx) => (
                                                        <option key={idx} value={idx}>{h ? `${h} (Col ${idx + 1})` : `Column ${idx + 1}`}</option>
                                                    ))}
                                                </Select>
                                            </Td>
                                        </Tr>
                                    ))}
                                </Tbody>
                            </Table>
                        </Box>

                        <HStack justify="flex-end" pt={4}>
                            <Button onClick={() => { setFile(null); setCsvData([]); setActiveStep(0); }}>Back</Button>
                            <Button colorScheme="brand" onClick={() => setActiveStep(2)} isDisabled={!mapping.name}>
                                Next Step
                            </Button>
                        </HStack>
                    </VStack>
                );
            case 2:
                return (
                    <VStack spacing={4} py={8}>
                        <Icon as={FiCheckCircle} boxSize={12} color="green.500" />
                        <Text fontSize="lg" fontWeight="bold">Ready to import!</Text>
                        <Text textAlign="center">
                            We will process <b>{csvData.length - headerRowIndex - 1}</b> rows from the CSV file.
                        </Text>
                        <Text fontSize="sm" color="gray.500" textAlign="center" maxW="80%">
                            Existing products with the same name will be updated with the new pricing and mapped fields.
                            New items will be automatically created.
                        </Text>
                        <HStack justify="flex-end" pt={4} w="full">
                            <Button onClick={() => setActiveStep(1)} isDisabled={isUploading}>Back</Button>
                            <Button colorScheme="green" onClick={processImport} isLoading={isUploading}>
                                Start Bulk Import
                            </Button>
                        </HStack>
                    </VStack>
                );
            default:
                return null;
        }
    };

    return (
        <Modal isOpen={isOpen} onClose={onClose} size="xl" scrollBehavior="inside">
            <ModalOverlay />
            <ModalContent>
                <ModalHeader>Import Product Data & Benchmarks</ModalHeader>
                <ModalCloseButton />
                <ModalBody pb={6}>
                    <HStack spacing={4} mb={8} justify="center">
                        {STEPS.map((step, index) => (
                            <React.Fragment key={index}>
                                <HStack>
                                    <Box
                                        w={8} h={8} borderRadius="full"
                                        bg={activeStep >= index ? "brand.500" : "gray.200"}
                                        color={activeStep >= index ? "white" : "gray.500"}
                                        display="flex" alignItems="center" justifyContent="center"
                                        fontWeight="bold" fontSize="sm"
                                    >
                                        {index + 1}
                                    </Box>
                                    <Text fontWeight={activeStep >= index ? "bold" : "normal"} color={activeStep >= index ? "brand.700" : "gray.500"}>
                                        {step}
                                    </Text>
                                </HStack>
                                {index < STEPS.length - 1 && <Box w={8} h="2px" bg={activeStep > index ? "brand.500" : "gray.200"} />}
                            </React.Fragment>
                        ))}
                    </HStack>

                    {renderStepContent()}
                </ModalBody>
            </ModalContent>
        </Modal>
    );
}

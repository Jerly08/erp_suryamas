import React, { useState, useEffect } from 'react';
import {
    Box,
    Button,
    FormControl,
    FormLabel,
    Input,
    VStack,
    HStack,
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
    useToast,
    Select,
    FormErrorMessage,
    Textarea,
    Card,
    CardBody,
    Divider,
    InputGroup,
    InputLeftAddon,
} from '@chakra-ui/react';
import { useForm, useFieldArray, Controller } from 'react-hook-form';
import { FiPlus, FiTrash2, FiSave } from 'react-icons/fi';
import { useTranslation } from '@/hooks/useTranslation';
import { accountService } from '@/services/accountService';
import { ssotJournalService } from '@/services/ssotJournalService';
import { Account } from '@/types/account';
import { useAuth } from '@/contexts/AuthContext';

// Helper function to format number with thousand separators
const formatNumberWithSeparator = (value: number | string): string => {
    if (value === '' || value === null || value === undefined) return '';
    const num = typeof value === 'string' ? parseFloat(value.replace(/\./g, '')) : value;
    if (isNaN(num) || num === 0) return '';
    return num.toLocaleString('id-ID');
};

// Helper function to parse formatted number back to number
const parseFormattedNumber = (value: string): number => {
    if (!value || value === '') return 0;
    // Remove all dots (thousand separators) and parse
    const cleaned = value.replace(/\./g, '').replace(/,/g, '.');
    const num = parseFloat(cleaned);
    return isNaN(num) ? 0 : num;
};

// Currency Input Component
interface CurrencyInputProps {
    value: number;
    onChange: (value: number) => void;
    placeholder?: string;
    size?: string;
}

const CurrencyInput: React.FC<CurrencyInputProps> = ({ value, onChange, placeholder = '0', size = 'sm' }) => {
    const [displayValue, setDisplayValue] = useState<string>('');

    // Update display value when external value changes
    useEffect(() => {
        if (value && value !== 0) {
            setDisplayValue(formatNumberWithSeparator(value));
        } else {
            setDisplayValue('');
        }
    }, [value]);

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const inputValue = e.target.value;

        // Allow only numbers and dots
        const cleanedInput = inputValue.replace(/[^\d]/g, '');

        if (cleanedInput === '') {
            setDisplayValue('');
            onChange(0);
            return;
        }

        // Parse and format with thousand separators
        const numValue = parseInt(cleanedInput, 10);
        if (!isNaN(numValue)) {
            setDisplayValue(numValue.toLocaleString('id-ID'));
            onChange(numValue);
        }
    };

    const handleBlur = () => {
        // Re-format on blur to ensure proper display
        if (value && value !== 0) {
            setDisplayValue(formatNumberWithSeparator(value));
        }
    };

    return (
        <Input
            value={displayValue}
            onChange={handleChange}
            onBlur={handleBlur}
            placeholder={placeholder}
            size={size}
            textAlign="right"
        />
    );
};

interface JournalLineForm {
    account_id: string; // use string for select value
    description: string;
    debit: number;
    credit: number;
}

interface ManualEntryFormData {
    entry_date: string;
    reference: string;
    description: string;
    notes: string;
    lines: JournalLineForm[];
}

interface ManualEntryFormProps {
    onSuccess: () => void;
    onCancel: () => void;
    initialData?: {
        id: number;
        entry_date: string;
        reference: string;
        description: string;
        notes?: string;
        lines?: any[];
    };
}

export default function ManualEntryForm({ onSuccess, onCancel, initialData }: ManualEntryFormProps) {
    const { t } = useTranslation();
    const toast = useToast();
    const { user } = useAuth();
    const [accounts, setAccounts] = useState<Account[]>([]);
    const [loading, setLoading] = useState(false);
    const [submitting, setSubmitting] = useState(false);

    const {
        register,
        control,
        handleSubmit,
        watch,
        setValue,
        formState: { errors },
    } = useForm<ManualEntryFormData>({
        defaultValues: {
            entry_date: initialData ? initialData.entry_date.split('T')[0] : new Date().toISOString().split('T')[0],
            reference: initialData?.reference || '',
            description: initialData?.description || '',
            notes: initialData?.notes || '',
            lines: initialData?.lines
                ? initialData.lines.map(l => ({
                    account_id: l.account_id.toString(),
                    description: l.description,
                    debit: l.debit_amount,
                    credit: l.credit_amount
                }))
                : [
                    { account_id: '', description: '', debit: 0, credit: 0 },
                    { account_id: '', description: '', debit: 0, credit: 0 },
                ],
        },
    });

    const { fields, append, remove } = useFieldArray({
        control,
        name: 'lines',
    });

    // Load accounts
    useEffect(() => {
        const loadAccounts = async () => {
            setLoading(true);
            try {
                // Get account hierarchy or list
                // We can use getAccountCatalog for faster loading if available, but getAccounts gives full info
                // Using getAccounts to ensure we have code and name
                const token = localStorage.getItem('token') || '';
                if (!token) return;

                const loadedAccounts = await accountService.getAccounts(token);
                // Sort by code
                loadedAccounts.sort((a, b) => a.code.localeCompare(b.code));
                setAccounts(loadedAccounts);
            } catch (error) {
                console.error('Failed to load accounts', error);
                toast({
                    title: t('common.error'),
                    description: 'Failed to load accounts',
                    status: 'error',
                    duration: 3000,
                });
            } finally {
                setLoading(false);
            }
        };
        loadAccounts();
    }, [toast, t]);

    const watchLines = watch('lines');

    const totalDebit = watchLines.reduce((sum, line) => sum + (Number(line.debit) || 0), 0);
    const totalCredit = watchLines.reduce((sum, line) => sum + (Number(line.credit) || 0), 0);
    const isBalanced = Math.abs(totalDebit - totalCredit) < 0.01;

    const isSubmittingRef = React.useRef(false);

    const onSubmit = async (data: ManualEntryFormData) => {
        if (isSubmittingRef.current) return;

        if (!isBalanced) {
            toast({
                title: t('common.error'),
                description: t('journal.validation.unbalanced'),
                status: 'error',
                duration: 3000,
            });
            return;
        }

        if (totalDebit === 0) {
            toast({
                title: t('common.error'),
                description: t('journal.validation.zero_amount'),
                status: 'error',
                duration: 3000,
            });
            return;
        }

        isSubmittingRef.current = true;
        setSubmitting(true);
        try {
            const formattedLines = data.lines.map(line => ({
                account_id: parseInt(line.account_id),
                description: line.description || data.description, // Use line desc or fallback to header desc
                debit_amount: Number(line.debit) || 0,
                credit_amount: Number(line.credit) || 0,
            })).filter(line => line.account_id && (line.debit_amount > 0 || line.credit_amount > 0));

            if (formattedLines.length < 2) {
                toast({
                    title: t('common.error'),
                    description: t('journal.validation.min_lines'),
                    status: 'error',
                    duration: 3000,
                });
                isSubmittingRef.current = false;
                setSubmitting(false);
                return;
            }

            const payload = {
                entry_date: new Date(data.entry_date).toISOString(),
                description: data.description,
                reference: data.reference,
                notes: data.notes,
                lines: formattedLines,
            };

            if (initialData?.id) {
                await ssotJournalService.updateJournalEntry(initialData.id, payload);
                toast({
                    title: t('common.success'),
                    description: t('journal.update_success'),
                    status: 'success',
                    duration: 3000,
                });
            } else {
                await ssotJournalService.createJournalEntry(payload);
                toast({
                    title: t('common.success'),
                    description: t('journal.create_success'),
                    status: 'success',
                    duration: 3000,
                });
            }

            onSuccess();
        } catch (error: any) {
            console.error('Submit error:', error);
            toast({
                title: t('common.error'),
                description: error.message || t('journal.create_error'),
                status: 'error',
                duration: 3000,
            });
            isSubmittingRef.current = false;
        } finally {
            setSubmitting(false);
            isSubmittingRef.current = false;
        }
    };

    const formatCurrency = (val: number) => {
        return new Intl.NumberFormat('id-ID', {
            style: 'currency',
            currency: 'IDR',
            minimumFractionDigits: 0
        }).format(val);
    };

    return (
        <Box as="form" onSubmit={handleSubmit(onSubmit)}>
            <VStack spacing={6} align="stretch">
                <Card variant="outline">
                    <CardBody>
                        <VStack spacing={4}>
                            <HStack w="full" spacing={4}>
                                <FormControl isRequired isInvalid={!!errors.entry_date}>
                                    <FormLabel>Date</FormLabel>
                                    <Input type="date" {...register('entry_date', { required: true })} />
                                    <FormErrorMessage>{t('journal.validation.date_required')}</FormErrorMessage>
                                </FormControl>
                                <FormControl isRequired isInvalid={!!errors.reference}>
                                    <FormLabel>Reference</FormLabel>
                                    <Input placeholder="e.g. JE-001" {...register('reference', { required: true })} />
                                    <FormErrorMessage>{t('journal.validation.reference_required')}</FormErrorMessage>
                                </FormControl>
                            </HStack>
                            <FormControl isRequired isInvalid={!!errors.description}>
                                <FormLabel>Description</FormLabel>
                                <Input placeholder="Journal description" {...register('description', { required: true })} />
                                <FormErrorMessage>{t('journal.validation.description_required')}</FormErrorMessage>
                            </FormControl>
                            <FormControl>
                                <FormLabel>Notes</FormLabel>
                                <Textarea placeholder="Optional notes" {...register('notes')} />
                            </FormControl>
                        </VStack>
                    </CardBody>
                </Card>

                <Card variant="outline">
                    <CardBody>
                        <Table size="sm">
                            <Thead>
                                <Tr>
                                    <Th width="30%">Account</Th>
                                    <Th width="25%">Description (Optional)</Th>
                                    <Th width="20%">Debit</Th>
                                    <Th width="20%">Credit</Th>
                                    <Th width="5%"></Th>
                                </Tr>
                            </Thead>
                            <Tbody>
                                {fields.map((field, index) => (
                                    <Tr key={field.id}>
                                        <Td>
                                            <Controller
                                                control={control}
                                                name={`lines.${index}.account_id`}
                                                rules={{ required: true }}
                                                render={({ field }) => (
                                                    <Select {...field} placeholder="Select Account" size="sm">
                                                        {accounts.map(acc => (
                                                            <option key={acc.id} value={acc.id}>
                                                                {acc.code} - {acc.name}
                                                            </option>
                                                        ))}
                                                    </Select>
                                                )}
                                            />
                                        </Td>
                                        <Td>
                                            <Input
                                                size="sm"
                                                {...register(`lines.${index}.description`)}
                                                placeholder="Line description"
                                            />
                                        </Td>
                                        <Td>
                                            <Controller
                                                control={control}
                                                name={`lines.${index}.debit`}
                                                render={({ field }) => (
                                                    <CurrencyInput
                                                        value={field.value || 0}
                                                        onChange={(val) => field.onChange(val)}
                                                        placeholder="0"
                                                        size="sm"
                                                    />
                                                )}
                                            />
                                        </Td>
                                        <Td>
                                            <Controller
                                                control={control}
                                                name={`lines.${index}.credit`}
                                                render={({ field }) => (
                                                    <CurrencyInput
                                                        value={field.value || 0}
                                                        onChange={(val) => field.onChange(val)}
                                                        placeholder="0"
                                                        size="sm"
                                                    />
                                                )}
                                            />
                                        </Td>
                                        <Td>
                                            <IconButton
                                                aria-label="Remove line"
                                                icon={<FiTrash2 />}
                                                size="xs"
                                                colorScheme="red"
                                                variant="ghost"
                                                onClick={() => remove(index)}
                                            />
                                        </Td>
                                    </Tr>
                                ))}
                            </Tbody>
                        </Table>
                        <Button
                            leftIcon={<FiPlus />}
                            size="sm"
                            variant="ghost"
                            mt={2}
                            onClick={() => append({ account_id: '', description: '', debit: 0, credit: 0 })}
                        >
                            Add Line
                        </Button>
                    </CardBody>
                </Card>

                {/* Totals Section */}
                <Box p={4} bg="gray.50" borderRadius="md" borderWidth="1px">
                    <HStack justify="space-between" align="center">
                        <Text fontWeight="bold">Total</Text>
                        <HStack spacing={8}>
                            <VStack align="end" spacing={0}>
                                <Text fontSize="xs" color="gray.500">Total Debit</Text>
                                <Text fontWeight="bold" color="green.600">{formatCurrency(totalDebit)}</Text>
                            </VStack>
                            <VStack align="end" spacing={0}>
                                <Text fontSize="xs" color="gray.500">Total Credit</Text>
                                <Text fontWeight="bold" color="red.600">{formatCurrency(totalCredit)}</Text>
                            </VStack>
                            <VStack align="end" spacing={0}>
                                <Text fontSize="xs" color="gray.500">Difference</Text>
                                <Text
                                    fontWeight="bold"
                                    color={isBalanced ? "green.500" : "red.500"}
                                >
                                    {formatCurrency(Math.abs(totalDebit - totalCredit))}
                                </Text>
                            </VStack>
                        </HStack>
                    </HStack>
                </Box>

                <HStack justify="flex-end" spacing={4}>
                    <Button onClick={onCancel}>Cancel</Button>
                    <Button
                        colorScheme="blue"
                        leftIcon={<FiSave />}
                        type="submit"
                        isLoading={submitting}
                        isDisabled={!isBalanced || totalDebit === 0}
                    >
                        Save Journal
                    </Button>
                </HStack>

            </VStack>
        </Box>
    );
}

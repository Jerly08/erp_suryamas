'use client';

import { useAuth } from '@/contexts/AuthContext';
import { useTranslation } from '@/hooks/useTranslation';

export type CompanyType = 'SERVICE' | 'TRADING' | 'MANUFACTURING';

export function useCompanyType() {
    const { user } = useAuth();
    const { t } = useTranslation();

    const companyType = (user?.company_type as CompanyType) || 'TRADING'; // Default to TRADING if not set

    const isService = companyType === 'SERVICE';
    const isTrading = companyType === 'TRADING';
    const isManufacturing = companyType === 'MANUFACTURING';

    // Get company-type-specific label
    const getLabel = (key: string): string => {
        const translated = t(`companyType.${companyType}.${key}`);
        // If translation key not found, fallback to generic key
        if (translated.startsWith('companyType.')) {
            return t(`navigation.${key}`) || key;
        }
        return translated;
    };

    return {
        companyType,
        isService,
        isTrading,
        isManufacturing,
        getLabel,
        // Helper: whether inventory tracking is applicable
        hasInventory: !isService,
    };
}

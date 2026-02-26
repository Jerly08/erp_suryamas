'use client';

import React from 'react';
import ProtectedModule from '@/components/common/ProtectedModule';
import BOMMasterData from '@/components/boms/BOMMasterData';
import CompanyTypeGuard from '@/components/common/CompanyTypeGuard';

export default function BOMsPage() {
    return (
        <ProtectedModule module="boms">
            <CompanyTypeGuard>
                <BOMMasterData />
            </CompanyTypeGuard>
        </ProtectedModule>
    );
}

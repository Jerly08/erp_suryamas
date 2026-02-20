'use client';

import React from 'react';
import ProtectedModule from '@/components/common/ProtectedModule';
import ProductCatalog from '@/components/products/ProductCatalog';

import CompanyTypeGuard from '@/components/common/CompanyTypeGuard';

export default function ProductsPage() {
  return (
    <ProtectedModule module="products">
      <CompanyTypeGuard>
        <ProductCatalog />
      </CompanyTypeGuard>
    </ProtectedModule>
  );
}

export interface BOMItem {
    id: number;
    bom_id: number;
    component_product_id: number;
    category_str: string;
    base_quantity: number;
    base_unit: string;
    notes: string;
    unit_price: number;
    requirement_quantity: number;
    total_cost: number;
    cost_percentage: number;
    component_product?: any; // The raw material from Product list
}

export interface BOM {
    id: number;
    code: string;
    target_product_id: number;
    target_quantity: number;
    name: string;
    description: string;
    total_cost: number;
    created_at: string;
    updated_at: string;
    target_product?: any;
    items: BOMItem[];
}

export interface BOMItemCreatePayload {
    component_product_id: number;
    category_str: string;
    base_quantity: number;
    base_unit: string;
    notes: string;
    unit_price: number;
    requirement_quantity: number;
}

export interface BOMCreateRequest {
    code: string;
    target_product_id: number;
    target_quantity: number;
    name: string;
    description: string;
    items: BOMItemCreatePayload[];
}

export interface ProjectBOMItem {
    id: number;
    project_bom_id: number;
    component_product_id: number;
    category_str: string;
    base_quantity: number;
    base_unit: string;
    notes: string;
    total_requirement: number;
    unit_price: number;
    total_cost: number;
    component_product?: any;
}

export interface ProjectBOM {
    id: number;
    project_id: number;
    bom_id: number;
    bom_name: string;
    target_product_id: number;
    quantity: number;
    unit_cost: number;
    total_cost: number;
    created_at: string;
    updated_at: string;
    bom_template?: BOM;
    target_product?: any;
    items: ProjectBOMItem[];
}

export interface ProjectBOMAddPayload {
    bom_id: number;
    quantity: number;
}

// --- Component Templates (reusable component presets for BOM) ---

export interface ComponentTemplateItem {
    id: number;
    template_id: number;
    component_product_id: number;
    category_str: string;
    requirement_quantity: number;
    base_unit: string;
    notes: string;
    component_product?: any;
}

export interface ComponentTemplate {
    id: number;
    name: string;
    description: string;
    items: ComponentTemplateItem[];
    created_at: string;
}

export interface ComponentTemplateCreateRequest {
    name: string;
    description: string;
    items: {
        component_product_id: number;
        category_str: string;
        requirement_quantity: number;
        base_unit: string;
        notes: string;
    }[];
}

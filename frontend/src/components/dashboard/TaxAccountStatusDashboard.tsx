import React, { useState, useEffect } from 'react';
import { 
  Card, 
  CardContent, 
  CardHeader, 
  CardTitle,
  Progress,
  Badge,
  Alert,
  AlertDescription,
  Button
} from '@/components/ui';
import { 
  CheckCircle, 
  AlertTriangle, 
  XCircle, 
  Settings,
  TrendingUp,
  DollarSign,
  CreditCard,
  Building
} from 'lucide-react';

interface AccountInfo {
  id: number;
  code: string;
  name: string;
  type: string;
  is_active: boolean;
}

interface AccountStatus {
  is_configured: boolean;
  account?: AccountInfo;
  warnings?: string[];
}

interface TaxAccountDashboard {
  is_fully_configured: boolean;
  health_score: number;
  sales_receivable: AccountStatus;
  sales_cash: AccountStatus;
  sales_bank: AccountStatus;
  sales_revenue: AccountStatus;
  sales_output_vat: AccountStatus;
  system_warnings: string[];
  last_updated?: string;
  updated_by?: {
    name: string;
  };
}

const TaxAccountStatusDashboard: React.FC = () => {
  const [dashboard, setDashboard] = useState<TaxAccountDashboard | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchDashboard();
  }, []);

  const fetchDashboard = async () => {
    try {
      const response = await fetch('/api/v1/tax-accounts/status');
      const data = await response.json();
      
      if (data.success) {
        setDashboard(data.data);
      }
    } catch (error) {
      console.error('Failed to fetch dashboard:', error);
    } finally {
      setLoading(false);
    }
  };

  const getStatusBadge = (status: AccountStatus) => {
    if (!status.is_configured) {
      return <Badge variant="destructive">Not Configured</Badge>;
    }
    
    if (status.warnings && status.warnings.length > 0) {
      return <Badge variant="secondary">Warning</Badge>;
    }
    
    return <Badge variant="default">Configured</Badge>;
  };

  const getStatusIcon = (status: AccountStatus) => {
    if (!status.is_configured) {
      return <XCircle className="h-4 w-4 text-red-500" />;
    }
    
    if (status.warnings && status.warnings.length > 0) {
      return <AlertTriangle className="h-4 w-4 text-yellow-500" />;
    }
    
    return <CheckCircle className="h-4 w-4 text-green-500" />;
  };

  const getHealthScoreColor = (score: number) => {
    if (score >= 80) return 'bg-green-500';
    if (score >= 60) return 'bg-yellow-500';
    return 'bg-red-500';
  };

  if (loading) {
    return (
      <Card>
        <CardContent className="flex items-center justify-center p-8">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900"></div>
        </CardContent>
      </Card>
    );
  }

  if (!dashboard) {
    return (
      <Card>
        <CardContent className="p-8">
          <Alert>
            <AlertTriangle className="h-4 w-4" />
            <AlertDescription>
              Unable to load tax account configuration status.
            </AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      {/* Health Score Overview */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center space-x-2">
            <Settings className="h-5 w-5" />
            <span>Tax Account Configuration Health</span>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-medium">Configuration Health</span>
                <span className="text-sm text-gray-500">{dashboard.health_score}%</span>
              </div>
              <Progress 
                value={dashboard.health_score} 
                className="h-3"
                // @ts-ignore - custom color class
                style={{
                  '--progress-background': getHealthScoreColor(dashboard.health_score)
                } as any}
              />
              <div className="mt-2 text-xs text-gray-500">
                {dashboard.is_fully_configured ? 'All required accounts configured' : 'Some accounts need configuration'}
              </div>
            </div>
            
            <div className="space-y-2">
              <div className="flex justify-between items-center">
                <span className="text-sm">Status:</span>
                <Badge variant={dashboard.is_fully_configured ? 'default' : 'destructive'}>
                  {dashboard.is_fully_configured ? 'Complete' : 'Incomplete'}
                </Badge>
              </div>
              {dashboard.last_updated && (
                <div className="text-xs text-gray-500">
                  Last updated: {dashboard.last_updated} by {dashboard.updated_by?.name || 'System'}
                </div>
              )}
            </div>
          </div>
          
          {/* System Warnings */}
          {dashboard.system_warnings.length > 0 && (
            <Alert className="mt-4">
              <AlertTriangle className="h-4 w-4" />
              <AlertDescription>
                <div className="font-medium mb-1">Configuration Issues:</div>
                <ul className="list-disc list-inside text-sm space-y-1">
                  {dashboard.system_warnings.map((warning, index) => (
                    <li key={index}>{warning}</li>
                  ))}
                </ul>
              </AlertDescription>
            </Alert>
          )}
        </CardContent>
      </Card>

      {/* Account Configuration Details */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Sales Transaction Accounts */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center space-x-2 text-base">
              <CreditCard className="h-4 w-4" />
              <span>Sales Transaction Accounts</span>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center justify-between p-2 border rounded">
              <div className="flex items-center space-x-2">
                {getStatusIcon(dashboard.sales_receivable)}
                <div>
                  <div className="text-sm font-medium">Receivable Account</div>
                  {dashboard.sales_receivable.account ? (
                    <div className="text-xs text-gray-500">
                      [{dashboard.sales_receivable.account.code}] {dashboard.sales_receivable.account.name}
                    </div>
                  ) : (
                    <div className="text-xs text-red-500">Not configured</div>
                  )}
                </div>
              </div>
              {getStatusBadge(dashboard.sales_receivable)}
            </div>
            
            <div className="flex items-center justify-between p-2 border rounded">
              <div className="flex items-center space-x-2">
                {getStatusIcon(dashboard.sales_cash)}
                <div>
                  <div className="text-sm font-medium">Cash Account</div>
                  {dashboard.sales_cash.account ? (
                    <div className="text-xs text-gray-500">
                      [{dashboard.sales_cash.account.code}] {dashboard.sales_cash.account.name}
                    </div>
                  ) : (
                    <div className="text-xs text-red-500">Not configured</div>
                  )}
                </div>
              </div>
              {getStatusBadge(dashboard.sales_cash)}
            </div>
            
            <div className="flex items-center justify-between p-2 border rounded">
              <div className="flex items-center space-x-2">
                {getStatusIcon(dashboard.sales_bank)}
                <div>
                  <div className="text-sm font-medium">Bank Account</div>
                  {dashboard.sales_bank.account ? (
                    <div className="text-xs text-gray-500">
                      [{dashboard.sales_bank.account.code}] {dashboard.sales_bank.account.name}
                    </div>
                  ) : (
                    <div className="text-xs text-red-500">Not configured</div>
                  )}
                </div>
              </div>
              {getStatusBadge(dashboard.sales_bank)}
            </div>
          </CardContent>
        </Card>

        {/* Revenue & Tax Accounts */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center space-x-2 text-base">
              <TrendingUp className="h-4 w-4" />
              <span>Revenue & Tax Accounts</span>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center justify-between p-2 border rounded">
              <div className="flex items-center space-x-2">
                {getStatusIcon(dashboard.sales_revenue)}
                <div>
                  <div className="text-sm font-medium">Revenue Account</div>
                  {dashboard.sales_revenue.account ? (
                    <div className="text-xs text-gray-500">
                      [{dashboard.sales_revenue.account.code}] {dashboard.sales_revenue.account.name}
                    </div>
                  ) : (
                    <div className="text-xs text-red-500">Not configured</div>
                  )}
                </div>
              </div>
              {getStatusBadge(dashboard.sales_revenue)}
            </div>
            
            <div className="flex items-center justify-between p-2 border rounded">
              <div className="flex items-center space-x-2">
                {getStatusIcon(dashboard.sales_output_vat)}
                <div>
                  <div className="text-sm font-medium">Output VAT Account</div>
                  {dashboard.sales_output_vat.account ? (
                    <div className="text-xs text-gray-500">
                      [{dashboard.sales_output_vat.account.code}] {dashboard.sales_output_vat.account.name}
                    </div>
                  ) : (
                    <div className="text-xs text-red-500">Not configured</div>
                  )}
                </div>
              </div>
              {getStatusBadge(dashboard.sales_output_vat)}
            </div>

            {/* Action Button */}
            <div className="pt-4 border-t">
              <Button 
                variant="outline" 
                size="sm" 
                className="w-full"
                onClick={() => window.location.href = '/settings/tax-accounts'}
              >
                <Settings className="h-4 w-4 mr-2" />
                Configure Accounts
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Account Usage Information */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center space-x-2">
            <Building className="h-4 w-4" />
            <span>How These Accounts Are Used</span>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
            <div>
              <h4 className="font-medium mb-2">Sales Transactions:</h4>
              <ul className="space-y-1 text-gray-600">
                <li>• <strong>Receivable Account:</strong> Records credit sales (customer owes money)</li>
                <li>• <strong>Cash Account:</strong> Records immediate cash payments</li>
                <li>• <strong>Bank Account:</strong> Records bank transfers and checks</li>
              </ul>
            </div>
            <div>
              <h4 className="font-medium mb-2">Revenue & Tax:</h4>
              <ul className="space-y-1 text-gray-600">
                <li>• <strong>Revenue Account:</strong> Records sales income</li>
                <li>• <strong>Output VAT Account:</strong> Records tax obligations (PPN Keluaran)</li>
              </ul>
            </div>
          </div>
          
          <div className="mt-4 p-3 bg-blue-50 rounded-lg">
            <div className="flex items-start space-x-2">
              <DollarSign className="h-4 w-4 text-blue-600 mt-0.5" />
              <div className="text-sm">
                <div className="font-medium text-blue-900">Example Transaction:</div>
                <div className="text-blue-700">
                  When you make a sale for $1,000 + $110 VAT:
                  <br />• Revenue Account gets credited $1,000
                  <br />• Output VAT Account gets credited $110
                  <br />• Cash/Bank/Receivable Account gets debited $1,110
                </div>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
};

export default TaxAccountStatusDashboard;

import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { auth } from './api/client.js'
import MainLayout from './layouts/MainLayout.jsx'
import PlatformLayout from './layouts/PlatformLayout.jsx'
import LoginPage from './pages/LoginPage.jsx'
import MaterialPage from './pages/MaterialPage.jsx'
import SupplierPage from './pages/SupplierPage.jsx'
import SupplierMaterialPage from './pages/SupplierMaterialPage.jsx'
import CustomerPage from './pages/CustomerPage.jsx'
import WarehousePage from './pages/WarehousePage.jsx'
import LocationPage from './pages/LocationPage.jsx'
import PurchaseOrderPage from './pages/PurchaseOrderPage.jsx'
import SalesOrderPage from './pages/SalesOrderPage.jsx'
import UsersPage from './pages/UsersPage.jsx'
import TenantsPage from './pages/TenantsPage.jsx'
import RolesPage from './pages/RolesPage.jsx'
import OperationLogPage from './pages/OperationLogPage.jsx'
import BOMPage from './pages/BOMPage.jsx'
import StockPage from './pages/StockPage.jsx'
import InventoryPage from './pages/InventoryPage.jsx'
import PlanningPage from './pages/PlanningPage.jsx'

// Business guard: business tenants only; the platform tenant is routed to the
// separate platform console.
function BusinessProtected() {
  if (!auth.token()) return <Navigate to="/login" replace />
  if (auth.user()?.tenant === 'platform') return <Navigate to="/admin" replace />
  return <MainLayout />
}

// Platform guard: only the platform tenant may open the console.
function PlatformProtected() {
  if (!auth.token()) return <Navigate to="/login" replace />
  if (auth.user()?.tenant !== 'platform') return <Navigate to="/" replace />
  return <PlatformLayout />
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />

        {/* 平台管理控制台（平台租户专属） */}
        <Route path="/admin" element={<PlatformProtected />}>
          <Route index element={<Navigate to="/admin/tenants" replace />} />
          <Route path="tenants" element={<TenantsPage />} />
          <Route path="roles" element={<RolesPage />} />
          <Route path="logs" element={<OperationLogPage />} />
        </Route>

        {/* 业务系统（租户） */}
        <Route element={<BusinessProtected />}>
          <Route index element={<MaterialPage />} />
          <Route path="materials" element={<MaterialPage />} />
          <Route path="suppliers" element={<SupplierPage />} />
          <Route path="supplier-material" element={<SupplierMaterialPage />} />
          <Route path="customers" element={<CustomerPage />} />
          <Route path="warehouses" element={<WarehousePage />} />
          <Route path="locations" element={<LocationPage />} />
          <Route path="purchase-orders" element={<PurchaseOrderPage />} />
          <Route path="sales-orders" element={<SalesOrderPage />} />
          <Route path="users" element={<UsersPage />} />
          <Route path="operation-logs" element={<OperationLogPage />} />
          <Route path="boms" element={<BOMPage />} />
          <Route path="stock" element={<StockPage />} />
          <Route path="inventory" element={<InventoryPage />} />
          <Route path="planning" element={<PlanningPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}

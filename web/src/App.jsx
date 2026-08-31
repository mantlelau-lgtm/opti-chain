import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Navigate } from 'react-router-dom'
import { auth } from './api/client.js'
import MainLayout from './layouts/MainLayout.jsx'
import LoginPage from './pages/LoginPage.jsx'
import MaterialPage from './pages/MaterialPage.jsx'
import SupplierPage from './pages/SupplierPage.jsx'
import CustomerPage from './pages/CustomerPage.jsx'
import WarehousePage from './pages/WarehousePage.jsx'
import LocationPage from './pages/LocationPage.jsx'
import PurchaseOrderPage from './pages/PurchaseOrderPage.jsx'
import SalesOrderPage from './pages/SalesOrderPage.jsx'
import UsersPage from './pages/UsersPage.jsx'
import TenantsPage from './pages/TenantsPage.jsx'
import BOMPage from './pages/BOMPage.jsx'
import StockPage from './pages/StockPage.jsx'
import InventoryPage from './pages/InventoryPage.jsx'
import PlanningPage from './pages/PlanningPage.jsx'

// Protected bounces anonymous visitors to /login before rendering the app.
function Protected() {
   if (!auth.token()) return <Navigate to="/login" replace />
   return <MainLayout />
}

// Front-end router: one route per module, all rendered inside MainLayout.
export default function App() {
   return (
     <BrowserRouter>
       <Routes>
         <Route path="/login" element={<LoginPage />} />
         <Route element={<Protected />}>
           <Route index element={<MaterialPage />} />
           <Route path="materials" element={<MaterialPage />} />
           <Route path="suppliers" element={<SupplierPage />} />
           <Route path="customers" element={<CustomerPage />} />
           <Route path="warehouses" element={<WarehousePage />} />
           <Route path="locations" element={<LocationPage />} />
           <Route path="purchase-orders" element={<PurchaseOrderPage />} />
           <Route path="sales-orders" element={<SalesOrderPage />} />
           <Route path="users" element={<UsersPage />} />
           <Route path="tenants" element={<TenantsPage />} />
           <Route path="boms" element={<BOMPage />} />
           <Route path="stock" element={<StockPage />} />
           <Route path="inventory" element={<InventoryPage />} />
           <Route path="planning" element={<PlanningPage />} />
         </Route>
       </Routes>
     </BrowserRouter>
   )
}

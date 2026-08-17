import { createRouter, createWebHistory } from 'vue-router'
import { baseUrl } from '@/api/client'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  // Dùng chính đường dẫn gốc mà máy chủ khai báo, nhờ vậy bộ định tuyến hoạt
  // động đúng kể cả khi panel nằm sau một đường dẫn bí mật.
  history: createWebHistory(baseUrl() + '/'),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { public: true },
    },
    {
      path: '/',
      component: () => import('@/layouts/MainLayout.vue'),
      children: [
        {
          path: '',
          name: 'dashboard',
          component: () => import('@/views/DashboardView.vue'),
        },
        {
          path: 'profile',
          name: 'profile',
          component: () => import('@/views/ProfileView.vue'),
        },
        {
          path: 'users',
          name: 'users',
          component: () => import('@/views/UsersView.vue'),
          meta: { adminOnly: true },
        },
        {
          path: 'audit',
          name: 'audit',
          component: () => import('@/views/AuditView.vue'),
          meta: { adminOnly: true },
        },
      ],
    },
    // Đường dẫn lạ đưa về trang chủ thay vì hiện màn hình trắng.
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()

  if (to.meta.public) {
    return auth.isAuthenticated ? { name: 'dashboard' } : true
  }

  if (!auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }

  // Mở lại trang với token đã lưu: cần nạp hồ sơ người dùng trước khi xét quyền.
  if (!auth.user && !(await auth.loadUser())) {
    return { name: 'login' }
  }

  if (to.meta.adminOnly && !auth.isAdmin) {
    return { name: 'dashboard' }
  }

  return true
})

export default router

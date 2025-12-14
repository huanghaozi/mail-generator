import { createRouter, createWebHistory } from 'vue-router'
import Login from '../views/Login.vue'
import Layout from '../views/Layout.vue'
import Domains from '../views/Domains.vue'
import Accounts from '../views/Accounts.vue'
import Logs from '../views/Logs.vue'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: Login
  },
  {
    path: '/',
    component: Layout,
    redirect: '/domains',
    children: [
      {
        path: 'domains',
        name: 'Domains',
        component: Domains
      },
      {
        path: 'accounts',
        name: 'Accounts',
        component: Accounts
      },
      {
        path: 'logs',
        name: 'Logs',
        component: Logs
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token')
  if (to.name !== 'Login' && !token) {
    next({ name: 'Login' })
  } else {
    next()
  }
})

export default router



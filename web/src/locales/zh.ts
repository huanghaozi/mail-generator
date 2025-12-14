export default {
  common: {
    add: '添加',
    delete: '删除',
    action: '操作',
    refresh: '刷新',
    createdAt: '创建时间',
    updatedAt: '更新时间',
    confirmDelete: '确认删除？',
    success: '成功',
    error: '错误',
    required: '必填',
    id: 'ID',
    description: '描述',
  },
  login: {
    title: '邮件转发系统登录',
    password: '密码',
    loginButton: '登录',
    passwordPlaceholder: '请输入密码！',
  },
  menu: {
    domains: '域名管理',
    accounts: '账号规则',
    logs: '转发日志',
  },
  domain: {
    addTitle: '添加域名',
    name: '域名',
    namePlaceholder: 'example.com',
    instruction: '说明：请为该域名添加指向本服务器的 MX 记录。',
    added: '域名已添加',
    deleted: '域名已删除',
  },
  account: {
    addTitle: '添加转发规则',
    pattern: '匹配模式 (正则)',
    patternPlaceholder: '^.*{"@"}example\\.com$',
    patternTip: '使用正则表达式。例如：^support{"@"}.*$ 或 ^.*{"@"}mydomain\\.com$',
    forwardTo: '转发至',
    forwardToPlaceholder: 'me{"@"}gmail.com',
    hitCount: '命中次数',
    added: '规则已添加',
    deleted: '规则已删除',
  },
  log: {
    from: '发件人',
    to: '收件人',
    subject: '主题',
    status: '状态',
    time: '时间',
  }
}


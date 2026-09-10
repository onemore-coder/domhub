<template>
  <div>
    <el-card shadow="never" class="section">
      <template #header>
        <div class="section-header">
          <span>证书申请</span>
          <el-tag type="warning" effect="plain" size="small">规划中 · 能力分期落地</el-tag>
        </div>
      </template>
      <el-alert
        type="info" :closable="false" show-icon
        title="本页面为证书申请能力的规划占位"
        description="后续将打通「申请 → 验证 → 签发 → 部署 → 监控」闭环：免费证书走 ACME 自动签发（复用已接入的云账号 DNS 权限自动完成验证），付费证书对接云厂商 SSL 证书服务，已有证书可直接上传统一纳管，签发后一键部署到云资源。"
      />
      <el-steps :active="0" align-center class="steps">
        <el-step title="选择证书" description="免费 ACME / 付费购买 / 上传导入" />
        <el-step title="域名验证" description="DNS TXT 自动添加（复用云账号）" />
        <el-step title="签发" description="Let's Encrypt / ZeroSSL / 云厂商" />
        <el-step title="部署" description="CDN / 负载均衡 / 主机一键部署" />
        <el-step title="监控" description="自动纳入左侧「证书监控」" />
      </el-steps>
    </el-card>

    <el-row :gutter="16">
      <el-col :span="8" v-for="item in plans" :key="item.title">
        <el-card shadow="never" class="section plan-card">
          <div class="plan-title">
            <el-icon :size="18"><component :is="item.icon" /></el-icon>
            <b>{{ item.title }}</b>
            <el-tag size="small" :type="item.tagType" effect="plain">{{ item.phase }}</el-tag>
          </div>
          <div class="plan-desc">{{ item.desc }}</div>
          <ul class="plan-list">
            <li v-for="p in item.points" :key="p">{{ p }}</li>
          </ul>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
const plans = [
  {
    title: '免费证书（ACME）',
    icon: 'Stamp',
    phase: '一期',
    tagType: 'primary',
    desc: '参考 domain-admin：ACME 协议自动申请 Let\'s Encrypt / ZeroSSL 免费证书，90 天有效期自动续期。',
    points: [
      'DNS-01 验证：复用已接入的云账号 DNS 权限，TXT 记录自动添加与清理',
      'HTTP-01 验证：适用于解析指向本机的场景',
      '泛域名证书：*.example.com 一张证书覆盖全部子域',
      '签发成功自动写入「证书监控」，到期前自动续期',
    ],
  },
  {
    title: '付费证书购买',
    icon: 'ShoppingCart',
    phase: '二期',
    tagType: 'warning',
    desc: '对接云厂商 SSL 证书服务（腾讯云 / 阿里云等），在 DomHub 内完成选型、下单与提交资料。',
    points: [
      'DV / OV / EV 证书类型与品牌对比展示',
      '下单后跟踪审核与签发进度',
      '复用云账号体系，无需重复登录各厂商控制台',
    ],
  },
  {
    title: '证书上传与部署',
    icon: 'Upload',
    phase: '三期',
    tagType: 'info',
    desc: '已有第三方证书（如自行购买或老证书）统一上传纳管，并一键部署到云资源。',
    points: [
      'PEM / KEY 上传，私钥加密存储（复用凭证加密体系）',
      '部署目标：CDN、CLB/SLB、Web 托管主机',
      '部署后自动纳入证书监控与到期告警',
    ],
  },
]
</script>

<style scoped>
.section {
  margin-bottom: 16px;
}
.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.steps {
  margin-top: 20px;
}
.plan-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 15px;
  margin-bottom: 8px;
}
.plan-desc {
  font-size: 13px;
  color: var(--el-text-color-regular);
  margin-bottom: 10px;
  line-height: 1.6;
}
.plan-list {
  margin: 0;
  padding-left: 18px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  line-height: 1.9;
}
</style>

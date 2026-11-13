<template>
  <div class="welcome empty">
    <img :src="logoPng" style="width: 110px" alt="RedisLens" />
    <h2 style="margin: 4px 0 0">RedisLens</h2>
    <div class="faint">让数据看得见、改得动、管得住</div>
    <div class="feature-cards">
      <div class="f-card card" v-for="f in features" :key="f.title">
        <Icon :name="f.icon" :size="18" :style="{ color: f.color }" />
        <div>
          <div class="f-title">{{ f.title }}</div>
          <div class="f-sub faint">{{ f.desc }}</div>
        </div>
      </div>
    </div>
    <div style="display: flex; gap: 10px; margin-top: 8px">
      <button class="btn primary" @click="store.view = 'connections'">
        <Icon name="plus" :size="15" /> 创建第一个连接
      </button>
      <button class="btn" @click="store.view = 'settings'">
        <Icon name="settings" :size="15" /> 偏好设置
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import Icon from '../components/Icon.vue'
import logoPng from '../assets/logo.png'
import { store } from '../lib/store'

const features = [
  { icon: 'key', title: '键空间浏览与编辑', desc: 'SCAN 游标安全浏览，十万键级无阻塞', color: 'var(--t-string)' },
  { icon: 'activity', title: '实时监控仪表盘', desc: 'QPS / 命中率 / 内存 / 网络曲线', color: 'var(--t-hash)' },
  { icon: 'terminal', title: '命令行控制台', desc: '高亮补全，危险命令强制拦截', color: 'var(--t-list)' },
  { icon: 'chart', title: '大 Key 分析', desc: '渐进扫描排行，类型与 TTL 分布', color: 'var(--t-zset)' },
  { icon: 'radio', title: '发布订阅', desc: '频道实时监听与消息发布', color: 'var(--t-stream)' },
  { icon: 'shield', title: '操作审计', desc: '本地留痕 90 天，可回溯查询', color: 'var(--t-set)' },
]
</script>

<style scoped>
.welcome {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 40px;
}
.feature-cards {
  display: grid;
  grid-template-columns: repeat(3, 220px);
  gap: 10px;
  margin-top: 20px;
}
.f-card {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  text-align: left;
}
.f-title { font-size: 13px; font-weight: 600; }
.f-sub { font-size: 11.5px; margin-top: 2px; line-height: 1.4; }
</style>

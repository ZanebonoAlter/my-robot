// https://nuxt.com/docs/api/configuration/nuxt-config
import tailwindcss from "@tailwindcss/vite";

export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  ssr: false,
  devtools: { enabled: true },
  ignore: ['app/_deprecated/**'],
  runtimeConfig: {
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE || 'http://localhost:5000/api',
    },
  },
  vite: {
    plugins: [
      tailwindcss(),
    ],
  },
  components: [
    { path: '~/components', pathPrefix: false },
  ],
  css: ['~/assets/css/main.css'],
  modules: ['motion-v/nuxt', '@pinia/nuxt'],
  app: {
    head: {
      title: 'Syntopica',
      htmlAttrs: {
        'data-theme': 'editorial',
      },
      link: [
        { rel: 'icon', type: 'image/png', href: '/favicon.png' },
        { rel: 'stylesheet', href: 'https://fonts.googleapis.com/css2?family=Noto+Serif+SC:wght@300;400;500;600;700&display=swap' },
      ],
      script: [
        {
          innerHTML: `(function(){try{var t=localStorage.getItem('syntopica-theme');if(t==='editorial'||t==='dark'){document.documentElement.setAttribute('data-theme',t)}else{document.documentElement.setAttribute('data-theme','editorial')}}catch(e){document.documentElement.setAttribute('data-theme','editorial')}})()`,
          tagPosition: 'head',
        },
      ],
    },
  },
})

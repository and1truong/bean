import {defineConfig} from '@playwright/test'
export default defineConfig({testDir:'.',testMatch:/.*\.spec\.ts/,fullyParallel:false,workers:1,retries:process.env.CI?1:0,reporter:[['list'],['html',{outputFolder:'artifacts/report',open:'never'}]],use:{trace:'retain-on-failure',screenshot:'only-on-failure',video:'retain-on-failure'},outputDir:'artifacts/results',timeout:30_000})

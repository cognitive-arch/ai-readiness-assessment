/** @type {import('next').NextConfig} */
const nextConfig = {
  // Explicitly expose env vars to the browser bundle.
  // This guarantees they are available even if .env.local is loaded
  // after the module cache has already been populated.
  env: {
    NEXT_PUBLIC_API_BASE_URL: process.env.NEXT_PUBLIC_API_BASE_URL,
  },
};

module.exports = nextConfig;

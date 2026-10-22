export default ({ config }) => ({
  ...config,
  updates: {
    ...config.updates,
    url: process.env.EXPO_PUBLIC_API_URL
      ? `${new URL(process.env.EXPO_PUBLIC_API_URL).origin}/api/v1/mobile/updates`
      : config.updates.url,
  },
});

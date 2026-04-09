import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  metadataBase: new URL(
    process.env.NEXT_PUBLIC_SITE_URL ?? "https://arrakis.dev"
  ),
  title: "Arrakis — Supply Chain Security",
  description:
    "Arrakis is a self-hosted, open-source supply chain security platform. Scan dependencies, detect vulnerabilities, and make open source safer.",
  openGraph: {
    title: "Arrakis — Know what you run.",
    description:
      "Self-hosted, open-source supply chain security. Scan your dependencies, detect CVEs, and flag risky packages before they reach production.",
    siteName: "Arrakis",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Arrakis — Know what you run.",
    description:
      "Self-hosted, open-source supply chain security. Scan your dependencies, detect CVEs, and flag risky packages before they reach production.",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link
          rel="preconnect"
          href="https://fonts.gstatic.com"
          crossOrigin="anonymous"
        />
        <link
          href="https://fonts.googleapis.com/css2?family=DotGothic16&family=Inter:wght@300;400;500;600&display=swap"
          rel="stylesheet"
        />
      </head>
      <body>{children}</body>
    </html>
  );
}

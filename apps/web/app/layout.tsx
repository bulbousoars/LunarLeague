import type { Metadata } from "next";
import "./globals.css";
import { Providers } from "./providers";

export const metadata: Metadata = {
  title: "Lunar League — your league, your rules",
  description:
    "Self-hosted, open-source fantasy sports for friends who don't want to be Yahoo's product.",
  manifest: "/manifest.webmanifest",
  themeColor: "#0b101a",
  icons: {
    icon: "/icon.svg",
    apple: "/icon.svg",
  },
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className="h-full">
      <body className="min-h-full antialiased">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}

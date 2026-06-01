import InstallationClient from "./InstallationClient";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Installation & Setup Guide - Bloc Hub",
  description: "Install the Bloc CLI toolchain on macOS, Linux, or Windows. Scan hardware memory limits and link GitHub developer keys in seconds.",
};

export default function InstallationPage() {
  return <InstallationClient />;
}

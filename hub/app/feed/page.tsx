import FeedClient from "./FeedClient";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Developer Stream - Bloc Hub",
  description: "Dynamic feed of local model optimization configurations and setup scripts submitted by engineers you follow.",
};

export default function FeedPage() {
  return <FeedClient />;
}

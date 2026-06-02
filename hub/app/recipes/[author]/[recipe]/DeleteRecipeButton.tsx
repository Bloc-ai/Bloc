"use client";

import { useAuth } from "@/context/AuthContext";
import { supabase } from "@/lib/supabase";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";
import { Trash2 } from "lucide-react";

interface DeleteRecipeButtonProps {
  creator: string;
  recipeName: string;
  isMock?: boolean;
}

export default function DeleteRecipeButton({ 
  creator, 
  recipeName,
  isMock = false
}: DeleteRecipeButtonProps) {
  const { user } = useAuth();
  const router = useRouter();
  const [isDeleting, setIsDeleting] = useState(false);

  // Only render the delete button if the logged-in user is the owner/creator
  if (!user || user.username !== creator) {
    return null;
  }

  const handleDelete = async () => {
    if (isMock) {
      toast.error("Cannot delete default registry recipe.", {
        description: "Default recipes are read-only and cannot be removed."
      });
      return;
    }

    const confirmed = window.confirm(
      `Are you sure you want to delete the recipe "${creator}/${recipeName}"? This action cannot be undone.`
    );
    if (!confirmed) return;

    if (!supabase) {
      toast.error("Database connection unavailable.");
      return;
    }

    setIsDeleting(true);
    const toastId = toast.loading("Deleting recipe from registry...");

    try {
      const { data, error } = await supabase
        .from("recipes")
        .delete()
        .eq("creator", creator)
        .eq("name", recipeName)
        .select();

      if (error) throw error;

      if (!data || data.length === 0) {
        throw new Error(
          "Permission denied or recipe not found. Please ensure you are logged in with the correct account."
        );
      }

      toast.dismiss(toastId);
      toast.success("Recipe deleted successfully.", {
        description: `Successfully removed ${creator}/${recipeName} from the registry.`
      });
      
      // Redirect to registry list and refresh page data
      router.push("/registry");
      router.refresh();
    } catch (err: any) {
      toast.dismiss(toastId);
      toast.error("Failed to delete recipe", {
        description: err.message || "An unexpected database error occurred."
      });
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <button
      onClick={handleDelete}
      disabled={isDeleting}
      className="flex items-center gap-1.5 h-7 px-3 border border-red-500/20 hover:border-red-500/40 bg-red-500/5 hover:bg-red-500/10 text-red-600 dark:text-red-400 rounded-md font-mono text-[9px] uppercase font-bold tracking-wider cursor-pointer transition-all duration-150 disabled:opacity-50 select-none shrink-0"
    >
      <Trash2 className="w-3 h-3" />
      {isDeleting ? "Deleting..." : "Delete Recipe"}
    </button>
  );
}

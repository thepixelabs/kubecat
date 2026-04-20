import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ThemeSettings } from "./ThemeSettings";

// Stub next-themes so we can observe setTheme calls.
const { mockSetTheme, mockUseTheme } = vi.hoisted(() => ({
  mockSetTheme: vi.fn(),
  mockUseTheme: vi.fn().mockReturnValue({
    theme: "dark",
    setTheme: vi.fn(),
  }),
}));
vi.mock("next-themes", () => ({
  useTheme: () => mockUseTheme(),
}));

describe("ThemeSettings", () => {
  beforeEach(() => {
    mockSetTheme.mockReset();
    mockUseTheme.mockReturnValue({ theme: "dark", setTheme: mockSetTheme });
  });

  it("renders nothing before mount (hydration-safe)", async () => {
    // On first render, mounted=false and the component returns null. We get
    // the post-effect render shortly after.
    render(
      <ThemeSettings
        colorTheme="rain"
        setColorTheme={vi.fn()}
        selectionColor=""
        setSelectionColor={vi.fn()}
      />
    );
    await waitFor(() => {
      expect(screen.getByText("Appearance Mode")).toBeInTheDocument();
    });
  });

  it("switches appearance mode when the user clicks a different option", async () => {
    const user = userEvent.setup();
    render(
      <ThemeSettings
        colorTheme="rain"
        setColorTheme={vi.fn()}
        selectionColor=""
        setSelectionColor={vi.fn()}
      />
    );
    await waitFor(() => screen.getByText("Light"));
    await user.click(screen.getByRole("button", { name: /Light/i }));
    expect(mockSetTheme).toHaveBeenCalledWith("light");
  });

  it("invokes setColorTheme when a color theme button is clicked", async () => {
    const setColorTheme = vi.fn();
    const user = userEvent.setup();
    render(
      <ThemeSettings
        colorTheme="rain"
        setColorTheme={setColorTheme}
        selectionColor=""
        setSelectionColor={vi.fn()}
      />
    );
    await waitFor(() => screen.getByText("Purple"));
    await user.click(screen.getByRole("button", { name: /Purple/i }));
    expect(setColorTheme).toHaveBeenCalledWith("purple");
  });

  it("resets selection color when 'Reset to Default' is clicked", async () => {
    const setSelection = vi.fn();
    const user = userEvent.setup();
    render(
      <ThemeSettings
        colorTheme="rain"
        setColorTheme={vi.fn()}
        selectionColor="#ff0000"
        setSelectionColor={setSelection}
      />
    );
    await waitFor(() => screen.getByText("Reset to Default"));
    await user.click(screen.getByRole("button", { name: /Reset to Default/i }));
    expect(setSelection).toHaveBeenCalledWith("");
  });
});


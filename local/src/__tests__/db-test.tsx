import React from "react";
import { fireEvent, render, waitFor } from "@testing-library/react-native";

import DatabaseTestScreen from "@/app/(tabs)/db";
import { database } from "@/model";

// The screen imports the sync module, but these UI tests do not exercise the
// real network/file-system sync path. Mocking it keeps the test focused on the
// WatermelonDB demo screen behavior.
jest.mock("@/sync/sync", () => ({
  syncSpace: jest.fn(),
}));

// Mock the WatermelonDB database instance used by the demo screen.
jest.mock("@/model", () => {
  const mockSubscribe = jest.fn((callback) => {
    callback([
      {
        id: "1",
        description: "离线测试：AA制午餐",
        amount: 4500,
        prepareMarkAsDeleted: jest.fn(),
      },
    ]);
    return { unsubscribe: jest.fn() };
  });

  return {
    database: {
      collections: {
        get: jest.fn(() => ({
          find: jest.fn().mockRejectedValue(new Error("not found")),
          query: jest.fn(() => ({
            observe: jest.fn(() => ({
              subscribe: mockSubscribe,
            })),
            fetch: jest.fn().mockResolvedValue([]),
          })),
          create: jest.fn(),
        })),
      },
      write: jest.fn(async (action) => {
        await action();
      }),
      batch: jest.fn(),
    },
  };
});

describe("DatabaseTestScreen", () => {
  it("renders the initial expense data", () => {
    const { getByText } = render(<DatabaseTestScreen />);

    expect(getByText("￥45.00")).toBeTruthy();
    expect(getByText("离线测试：AA制午餐")).toBeTruthy();
  });

  it("calls database.write when the add button is pressed", async () => {
    const { getAllByRole } = render(<DatabaseTestScreen />);
    const addButton = getAllByRole("button")[0];

    fireEvent.press(addButton);

    await waitFor(() => {
      expect(database.write).toHaveBeenCalled();
    });
  });

  it("calls database.batch when the clear button is pressed", async () => {
    const { getAllByRole } = render(<DatabaseTestScreen />);
    const clearButton = getAllByRole("button")[2];

    fireEvent.press(clearButton);

    await waitFor(() => {
      expect(database.write).toHaveBeenCalled();
      expect(database.batch).toHaveBeenCalled();
    });
  });
});

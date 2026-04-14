import React from "react";
import { render, fireEvent, waitFor } from "@testing-library/react-native";
import DatabaseTestScreen from "@/app/(tabs)/db";
import { database } from "@/model";

// 1. 模拟 @/model 中的 database 实例
jest.mock("@/model", () => {
  const mockSubscribe = jest.fn((callback) => {
    // 模拟初始加载时，数据库里有一条离线记录
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
            fetch: jest.fn().mockResolvedValue([]), // 模拟查出所有数据
          })),
          create: jest.fn(),
        })),
      },
      write: jest.fn(async (action) => {
        // 模拟执行 write 闭包里的逻辑
        await action();
      }),
      batch: jest.fn(),
    },
  };
});

describe("DatabaseTestScreen - 旅游工具箱离线记账测试", () => {
  it("应该能正确渲染页面标题和初始的账单数据", () => {
    const { getByText } = render(<DatabaseTestScreen />);

    // 验证 UI 是否正常渲染
    expect(getByText("WatermelonDB 离线测试室")).toBeTruthy();
    // 验证是否读到了 Mock 的那一条数据（4500分 = 45.00元）
    expect(getByText("￥45.00")).toBeTruthy();
    expect(getByText("离线测试：AA制午餐")).toBeTruthy();
  });

  it('点击"记一笔账"时，应该调用 database.write', async () => {
    const { getByText } = render(<DatabaseTestScreen />);
    const addButton = getByText("记一笔账");

    // 模拟用户点击操作
    fireEvent.press(addButton);

    // 验证是否触发了数据库的写入事务
    await waitFor(() => {
      expect(database.write).toHaveBeenCalled();
    });
  });

  it('点击"清空全部"时，应该调用 database.batch 执行批量删除', async () => {
    const { getByText } = render(<DatabaseTestScreen />);
    const clearButton = getByText("清空全部");

    fireEvent.press(clearButton);

    await waitFor(() => {
      // 首先要调用 write 开启事务
      expect(database.write).toHaveBeenCalled();
      // 然后我们 mock 的逻辑里应该会走到 batch 进行批量删除
      expect(database.batch).toHaveBeenCalled();
    });
  });
});

"use client"

import { useState } from "react"
import { Button } from "@/components/ui/button"

interface Strategy {
  id: string
  name: string
  description: string
  status: "active" | "inactive" | "error"
  symbols: string[]
  createdAt: string
}

export default function StrategiesPage() {
  const [strategies] = useState<Strategy[]>([
    {
      id: "1",
      name: "EMA Cross Strategy",
      description: "Simple moving average crossover strategy",
      status: "active",
      symbols: ["AAPL", "GOOGL"],
      createdAt: "2024-01-15",
    },
    {
      id: "2",
      name: "RSI Reversal Strategy",
      description: "RSI-based mean reversion strategy",
      status: "inactive",
      symbols: ["MSFT"],
      createdAt: "2024-01-10",
    },
  ])

  const [selectedStrategy, setSelectedStrategy] = useState<Strategy | null>(null)

  return (
    <div className="container mx-auto px-4 py-8">
      <div className="flex justify-between items-center mb-8">
        <h1 className="text-3xl font-bold">Strategy Center</h1>
        <Button>Create New Strategy</Button>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Strategy List */}
        <div className="lg:col-span-1">
          <div className="bg-white rounded-lg shadow p-6">
            <h2 className="text-xl font-semibold mb-4">Strategies</h2>
            <div className="space-y-3">
              {strategies.map((strategy) => (
                <div
                  key={strategy.id}
                  className={`p-3 rounded border cursor-pointer transition-colors ${
                    selectedStrategy?.id === strategy.id
                      ? "border-blue-500 bg-blue-50"
                      : "border-gray-200 hover:border-gray-300"
                  }`}
                  onClick={() => setSelectedStrategy(strategy)}
                >
                  <div className="flex justify-between items-start">
                    <div>
                      <h3 className="font-medium">{strategy.name}</h3>
                      <p className="text-sm text-gray-600 mt-1">
                        {strategy.description}
                      </p>
                    </div>
                    <span
                      className={`px-2 py-1 rounded-full text-xs ${
                        strategy.status === "active"
                          ? "bg-green-100 text-green-800"
                          : strategy.status === "error"
                          ? "bg-red-100 text-red-800"
                          : "bg-gray-100 text-gray-800"
                      }`}
                    >
                      {strategy.status}
                    </span>
                  </div>
                  <div className="mt-2 text-xs text-gray-500">
                    Symbols: {strategy.symbols.join(", ")}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Strategy Details */}
        <div className="lg:col-span-2">
          {selectedStrategy ? (
            <div className="bg-white rounded-lg shadow p-6">
              <div className="flex justify-between items-start mb-6">
                <div>
                  <h2 className="text-2xl font-bold">{selectedStrategy.name}</h2>
                  <p className="text-gray-600 mt-1">
                    {selectedStrategy.description}
                  </p>
                </div>
                <div className="flex space-x-2">
                  <Button variant="outline">Edit</Button>
                  <Button variant="outline">Backtest</Button>
                  <Button
                    variant={selectedStrategy.status === "active" ? "destructive" : "default"}
                  >
                    {selectedStrategy.status === "active" ? "Stop" : "Start"}
                  </Button>
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                {/* Strategy Info */}
                <div>
                  <h3 className="text-lg font-semibold mb-3">Strategy Information</h3>
                  <div className="space-y-2">
                    <div className="flex justify-between">
                      <span className="text-gray-600">Status:</span>
                      <span
                        className={`px-2 py-1 rounded-full text-xs ${
                          selectedStrategy.status === "active"
                            ? "bg-green-100 text-green-800"
                            : selectedStrategy.status === "error"
                            ? "bg-red-100 text-red-800"
                            : "bg-gray-100 text-gray-800"
                        }`}
                      >
                        {selectedStrategy.status}
                      </span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-gray-600">Symbols:</span>
                      <span>{selectedStrategy.symbols.join(", ")}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-gray-600">Created:</span>
                      <span>{selectedStrategy.createdAt}</span>
                    </div>
                  </div>
                </div>

                {/* Performance */}
                <div>
                  <h3 className="text-lg font-semibold mb-3">Performance</h3>
                  <div className="space-y-2">
                    <div className="flex justify-between">
                      <span className="text-gray-600">Total Return:</span>
                      <span className="text-green-600 font-medium">+12.5%</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-gray-600">Sharpe Ratio:</span>
                      <span className="font-medium">1.85</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-gray-600">Max Drawdown:</span>
                      <span className="text-red-600 font-medium">-8.2%</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-gray-600">Win Rate:</span>
                      <span className="font-medium">65.2%</span>
                    </div>
                  </div>
                </div>
              </div>

              {/* Recent Trades */}
              <div className="mt-8">
                <h3 className="text-lg font-semibold mb-3">Recent Trades</h3>
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b">
                        <th className="text-left py-2">Time</th>
                        <th className="text-left py-2">Symbol</th>
                        <th className="text-left py-2">Side</th>
                        <th className="text-left py-2">Quantity</th>
                        <th className="text-left py-2">Price</th>
                        <th className="text-left py-2">PnL</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr className="border-b">
                        <td className="py-2">2024-01-15 14:30</td>
                        <td className="py-2">AAPL</td>
                        <td className="py-2 text-green-600">BUY</td>
                        <td className="py-2">100</td>
                        <td className="py-2">$185.50</td>
                        <td className="py-2 text-green-600">+$150.00</td>
                      </tr>
                      <tr className="border-b">
                        <td className="py-2">2024-01-15 13:45</td>
                        <td className="py-2">GOOGL</td>
                        <td className="py-2 text-red-600">SELL</td>
                        <td className="py-2">50</td>
                        <td className="py-2">$142.75</td>
                        <td className="py-2 text-red-600">-$75.00</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          ) : (
            <div className="bg-white rounded-lg shadow p-6">
              <div className="text-center text-gray-500">
                <p>Select a strategy to view details</p>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
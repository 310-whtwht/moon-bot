import { NextResponse } from 'next/server';

// モックデータ
const mockStrategies = [
  {
    id: '1',
    name: 'EMA クロス戦略',
    description: '短期と長期のEMAのクロスオーバーを利用した戦略',
    author: 'システム管理者',
    is_public: true,
    created_at: '2024-01-15T10:00:00Z',
    updated_at: '2024-01-15T10:00:00Z',
    status: 'active',
    pnl: 15420.5,
    trades: 342,
    winRate: 68.5,
    lastTrade: '2024-01-15T14:30:00Z'
  },
  {
    id: '2',
    name: 'RSI リバーサル戦略',
    description: 'RSIの過買い・過売りを利用したリバーサル戦略',
    author: 'システム管理者',
    is_public: true,
    created_at: '2024-01-14T09:00:00Z',
    updated_at: '2024-01-14T09:00:00Z',
    status: 'paused',
    pnl: 8750.25,
    trades: 156,
    winRate: 72.3,
    lastTrade: '2024-01-14T16:45:00Z'
  },
  {
    id: '3',
    name: 'ブレイクアウト戦略',
    description: 'サポート・レジスタンスラインのブレイクアウトを狙う戦略',
    author: 'システム管理者',
    is_public: false,
    created_at: '2024-01-13T11:30:00Z',
    updated_at: '2024-01-13T11:30:00Z',
    status: 'stopped',
    pnl: -1250.75,
    trades: 89,
    winRate: 45.2,
    lastTrade: '2024-01-13T13:20:00Z'
  }
];

export async function GET() {
  // 遅延をシミュレート
  await new Promise(resolve => setTimeout(resolve, 500));
  
  return NextResponse.json({
    success: true,
    data: mockStrategies,
    message: '戦略の取得に成功しました'
  });
}

export async function POST(request: Request) {
  try {
    const body = await request.json();
    
    // 新しい戦略を作成
    const newStrategy = {
      id: Date.now().toString(),
      name: body.name,
      description: body.description || '',
      author: 'システム管理者',
      is_public: body.is_public || false,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      status: 'active',
      pnl: 0,
      trades: 0,
      winRate: 0,
      lastTrade: new Date().toISOString()
    };

    // 遅延をシミュレート
    await new Promise(resolve => setTimeout(resolve, 800));

    return NextResponse.json({
      success: true,
      data: newStrategy,
      message: '戦略の作成に成功しました'
    });
  } catch (error) {
    return NextResponse.json(
      {
        success: false,
        message: '戦略の作成に失敗しました'
      },
      { status: 400 }
    );
  }
}
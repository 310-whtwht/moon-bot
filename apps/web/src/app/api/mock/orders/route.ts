import { NextResponse } from 'next/server';

// モックデータ
const mockOrders = [
  {
    id: '1',
    symbol: 'AAPL',
    side: 'buy',
    status: 'filled',
    quantity: 100,
    price: 185.50,
    created_at: '2024-01-15T14:30:00Z'
  },
  {
    id: '2',
    symbol: 'TSLA',
    side: 'sell',
    status: 'pending',
    quantity: 50,
    price: 245.75,
    created_at: '2024-01-15T14:25:00Z'
  },
  {
    id: '3',
    symbol: 'MSFT',
    side: 'buy',
    status: 'cancelled',
    quantity: 75,
    price: 380.25,
    created_at: '2024-01-15T14:20:00Z'
  },
  {
    id: '4',
    symbol: 'GOOGL',
    side: 'sell',
    status: 'filled',
    quantity: 25,
    price: 142.80,
    created_at: '2024-01-15T14:15:00Z'
  },
  {
    id: '5',
    symbol: 'AMZN',
    side: 'buy',
    status: 'pending',
    quantity: 200,
    price: 155.90,
    created_at: '2024-01-15T14:10:00Z'
  }
];

export async function GET(request: Request) {
  const { searchParams } = new URL(request.url);
  const limit = searchParams.get('limit');
  
  // 遅延をシミュレート
  await new Promise(resolve => setTimeout(resolve, 300));
  
  let orders = mockOrders;
  if (limit) {
    orders = mockOrders.slice(0, parseInt(limit));
  }
  
  return NextResponse.json({
    success: true,
    data: orders,
    message: '注文の取得に成功しました'
  });
}

export async function POST(request: Request) {
  try {
    const body = await request.json();
    
    // 新しい注文を作成
    const newOrder = {
      id: Date.now().toString(),
      symbol: body.symbol,
      side: body.side,
      status: 'pending',
      quantity: body.quantity,
      price: body.price,
      created_at: new Date().toISOString()
    };

    // 遅延をシミュレート
    await new Promise(resolve => setTimeout(resolve, 600));

    return NextResponse.json({
      success: true,
      data: newOrder,
      message: '注文の作成に成功しました'
    });
  } catch (error) {
    return NextResponse.json(
      {
        success: false,
        message: '注文の作成に失敗しました'
      },
      { status: 400 }
    );
  }
}